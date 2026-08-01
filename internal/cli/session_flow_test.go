package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"synocli/internal/config"
)

// fakeDSM is a minimal DSM API server for exercising the withSession state
// machine: discovery, login/logout, and a couple of File Station calls.
type fakeDSM struct {
	mu          sync.Mutex
	loginCount  int
	logoutCount int
	// sharesSIDs records the _sid of every list_share call, in order.
	sharesSIDs []string
	// expiredSIDs answer any entry.cgi call with code 119 (SID not found).
	expiredSIDs map[string]bool
	// copyStartCount counts CopyMove start calls.
	copyStartCount int
	// copyStatusCode is the error code every CopyMove status call returns
	// (0 means success/finished).
	copyStatusCode int
	// getinfoFiles overrides the List getinfo response entries when non-nil.
	getinfoFiles []any

	server *httptest.Server
}

func newFakeDSM(t *testing.T) *fakeDSM {
	t.Helper()
	f := &fakeDSM{expiredSIDs: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/webapi/query.cgi", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, map[string]any{
			"SYNO.API.Auth":              map[string]any{"path": "auth.cgi", "minVersion": 1, "maxVersion": 6},
			"SYNO.DownloadStation2.Task": map[string]any{"path": "entry.cgi", "minVersion": 1, "maxVersion": 2},
			"SYNO.FileStation.List":      map[string]any{"path": "entry.cgi", "minVersion": 1, "maxVersion": 2},
			"SYNO.FileStation.CopyMove":  map[string]any{"path": "entry.cgi", "minVersion": 1, "maxVersion": 3},
		})
	})
	mux.HandleFunc("/webapi/auth.cgi", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Query().Get("method") {
		case "login":
			f.loginCount++
			writeOK(w, map[string]any{"sid": fmt.Sprintf("sid-%d", f.loginCount)})
		case "logout":
			f.logoutCount++
			writeOK(w, nil)
		default:
			writeErrCode(w, 103)
		}
	})
	mux.HandleFunc("/webapi/entry.cgi", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		q := r.URL.Query()
		sid := q.Get("_sid")
		if f.expiredSIDs[sid] {
			writeErrCode(w, 119)
			return
		}
		switch q.Get("api") {
		case "SYNO.FileStation.List":
			switch q.Get("method") {
			case "list_share":
				f.sharesSIDs = append(f.sharesSIDs, sid)
				writeOK(w, map[string]any{"shares": []any{}, "total": 0})
			case "getinfo":
				files := f.getinfoFiles
				if files == nil {
					files = []any{map[string]any{"path": "/dest", "isdir": true}}
				}
				writeOK(w, map[string]any{"files": files})
			default:
				writeErrCode(w, 103)
			}
		case "SYNO.FileStation.CopyMove":
			switch q.Get("method") {
			case "start":
				f.copyStartCount++
				writeOK(w, map[string]any{"taskid": "FileStation_copy_task"})
			case "status":
				if f.copyStatusCode != 0 {
					writeErrCode(w, f.copyStatusCode)
					return
				}
				writeOK(w, map[string]any{"finished": true})
			default:
				writeErrCode(w, 103)
			}
		default:
			writeErrCode(w, 102)
		}
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func writeOK(w http.ResponseWriter, data any) {
	resp := map[string]any{"success": true}
	if data != nil {
		resp["data"] = data
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func writeErrCode(w http.ResponseWriter, code int) {
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"code": code}})
}

// writeTestConfig writes a 0600 config file and returns its path.
func writeTestConfig(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	root, ac := newRootCmd(strings.NewReader(""), &out, &errOut)
	root.SetArgs(args)
	err = runRoot(root, ac, &errOut)
	return out.String(), errOut.String(), err
}

func TestWithSessionReusesCachedSID(t *testing.T) {
	f := newFakeDSM(t)
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=admin",
		"password=pw",
		"reuse_session=true",
	)
	sessPath := config.SessionPathFromConfig(cfg)
	if err := config.WriteSession(sessPath, config.Session{SID: "cachedSID", Endpoint: f.server.URL, User: "admin"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	stdout, _, err := runCLI(t, "--config", cfg, "--json", "fs", "shares")
	if err != nil {
		t.Fatalf("fs shares: %v", err)
	}
	if f.loginCount != 0 {
		t.Fatalf("expected no login with valid cached session, got %d", f.loginCount)
	}
	if len(f.sharesSIDs) != 1 || f.sharesSIDs[0] != "cachedSID" {
		t.Fatalf("expected cached SID to be used, got %v", f.sharesSIDs)
	}
	if !strings.Contains(stdout, `"ok":true`) && !strings.Contains(stdout, `"ok": true`) {
		t.Fatalf("expected ok envelope, got %q", stdout)
	}
}

func TestWithSessionIgnoresCachedSIDForDifferentEndpoint(t *testing.T) {
	f := newFakeDSM(t)
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=admin",
		"password=pw",
		"reuse_session=true",
	)
	sessPath := config.SessionPathFromConfig(cfg)
	if err := config.WriteSession(sessPath, config.Session{SID: "otherNASsid", Endpoint: "https://other-nas:5001", User: "admin"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	if _, _, err := runCLI(t, "--config", cfg, "--json", "fs", "shares"); err != nil {
		t.Fatalf("fs shares: %v", err)
	}
	if f.loginCount != 1 {
		t.Fatalf("expected fresh login for endpoint mismatch, got %d logins", f.loginCount)
	}
	for _, sid := range f.sharesSIDs {
		if sid == "otherNASsid" {
			t.Fatal("cached SID for a different endpoint must never be sent")
		}
	}
}

func TestWithSessionIgnoresCachedSIDForDifferentUser(t *testing.T) {
	f := newFakeDSM(t)
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=alice",
		"password=pw",
		"reuse_session=true",
	)
	sessPath := config.SessionPathFromConfig(cfg)
	if err := config.WriteSession(sessPath, config.Session{SID: "bobSID", Endpoint: f.server.URL, User: "bob"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	if _, _, err := runCLI(t, "--config", cfg, "--json", "fs", "shares"); err != nil {
		t.Fatalf("fs shares: %v", err)
	}
	if f.loginCount != 1 {
		t.Fatalf("expected fresh login for user mismatch, got %d logins", f.loginCount)
	}
	if len(f.sharesSIDs) != 1 || f.sharesSIDs[0] == "bobSID" {
		t.Fatalf("cached SID of a different user must not be reused, got %v", f.sharesSIDs)
	}
}

func TestWithSessionExpiredSIDRetriesOnce(t *testing.T) {
	f := newFakeDSM(t)
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=admin",
		"password=pw",
		"reuse_session=true",
	)
	sessPath := config.SessionPathFromConfig(cfg)
	if err := config.WriteSession(sessPath, config.Session{SID: "staleSID", Endpoint: f.server.URL, User: "admin"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}
	f.expiredSIDs["staleSID"] = true

	if _, _, err := runCLI(t, "--config", cfg, "--json", "fs", "shares"); err != nil {
		t.Fatalf("fs shares after expiry: %v", err)
	}
	if f.loginCount != 1 {
		t.Fatalf("expected exactly one re-login, got %d", f.loginCount)
	}
	if len(f.sharesSIDs) != 1 || f.sharesSIDs[0] != "sid-1" {
		t.Fatalf("expected retry with fresh SID, got %v", f.sharesSIDs)
	}
	// The fresh SID must be persisted for the next invocation.
	sess, err := config.LoadSession(sessPath)
	if err != nil || sess.SID != "sid-1" {
		t.Fatalf("expected persisted sid-1, got %+v err=%v", sess, err)
	}
	// A persisted session must not be logged out on exit.
	if f.logoutCount != 0 {
		t.Fatalf("persisted session must not be logged out, got %d logouts", f.logoutCount)
	}
}

func TestWithSessionLogsOutWhenNotReusing(t *testing.T) {
	f := newFakeDSM(t)
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=admin",
		"password=pw",
	)

	if _, _, err := runCLI(t, "--config", cfg, "--json", "fs", "shares"); err != nil {
		t.Fatalf("fs shares: %v", err)
	}
	if f.loginCount != 1 {
		t.Fatalf("expected one login, got %d", f.loginCount)
	}
	if f.logoutCount != 1 {
		t.Fatalf("expected logout when reuse_session is off, got %d", f.logoutCount)
	}
}

func TestWithSessionAdoptsCachedUserForWhoami(t *testing.T) {
	f := newFakeDSM(t)
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"reuse_session=true",
	)
	sessPath := config.SessionPathFromConfig(cfg)
	if err := config.WriteSession(sessPath, config.Session{SID: "cachedSID", Endpoint: f.server.URL, User: "admin"}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	stdout, _, err := runCLI(t, "--config", cfg, "--json", "auth", "whoami")
	if err != nil {
		t.Fatalf("auth whoami: %v", err)
	}
	if f.loginCount != 0 {
		t.Fatalf("expected cached session, got %d logins", f.loginCount)
	}
	if !strings.Contains(stdout, `"user":"admin"`) && !strings.Contains(stdout, `"user": "admin"`) {
		t.Fatalf("whoami should report the cached session user, got %q", stdout)
	}
}

func TestWithSessionDoesNotRetryAfterCommittedMutation(t *testing.T) {
	f := newFakeDSM(t)
	f.copyStatusCode = 106 // session expires mid-poll, after the copy started
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=admin",
		"password=pw",
		"reuse_session=true",
	)

	_, _, err := runCLI(t, "--config", cfg, "--json", "fs", "copy", "/src/file", "--to", "/dest", "--interval", "10ms")
	if err == nil {
		t.Fatal("expected session-expiry error to surface")
	}
	if f.copyStartCount != 1 {
		t.Fatalf("copy must not be started twice after mid-poll expiry, got %d starts", f.copyStartCount)
	}
	if f.loginCount != 1 {
		t.Fatalf("expected no re-login after committed mutation, got %d logins", f.loginCount)
	}
}
