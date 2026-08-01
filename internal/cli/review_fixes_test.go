package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONEnvelopeForEarlyValidationErrors(t *testing.T) {
	stdout, _, err := runCLI(t, "--json", "ds", "wait", "dbid_1", "--interval", "-1s")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(stdout, `"ok":false`) && !strings.Contains(stdout, `"ok": false`) {
		t.Fatalf("expected error envelope on stdout in --json mode, got %q", stdout)
	}
	if !strings.Contains(stdout, "validation_error") {
		t.Fatalf("expected validation_error code in envelope, got %q", stdout)
	}
}

func TestJSONEnvelopeForCobraUsageErrors(t *testing.T) {
	stdout, _, err := runCLI(t, "--json", "fs", "md5", "a", "b")
	if err == nil {
		t.Fatal("expected arg-count error")
	}
	if !strings.Contains(stdout, `"ok":false`) && !strings.Contains(stdout, `"ok": false`) {
		t.Fatalf("expected error envelope on stdout, got %q", stdout)
	}
}

func TestMD5AndExtractRejectExtraArgs(t *testing.T) {
	for _, args := range [][]string{
		{"fs", "md5", "fileA", "fileB"},
		{"fs", "extract", "a.zip", "b.zip", "--to", "/dest"},
	} {
		t.Run(strings.Join(args[:2], "_"), func(t *testing.T) {
			_, _, err := runCLI(t, args...)
			if err == nil {
				t.Fatalf("%v must reject extra positional args", args)
			}
			if !strings.Contains(err.Error(), "accepts 1 arg") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestMaxWaitRejectsNegative(t *testing.T) {
	for _, args := range [][]string{
		{"ds", "wait", "dbid_1", "--max-wait", "-1s"},
		{"fs", "md5", "file", "--max-wait", "-1s"},
		{"fs", "copy", "/a", "--to", "/b", "--max-wait", "-1s"},
		{"fs", "search", "/a", "--pattern", "x", "--max-wait", "-1s"},
	} {
		t.Run(strings.Join(args[:2], "_"), func(t *testing.T) {
			_, _, err := runCLI(t, args...)
			if err == nil {
				t.Fatalf("%v must reject negative --max-wait", args)
			}
			if !strings.Contains(err.Error(), "--max-wait must be >= 0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFSGetSurfacesPerEntryErrorCode(t *testing.T) {
	f := newFakeDSM(t)
	f.getinfoFiles = []any{map[string]any{"path": "/no/such/file", "code": 408}}
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=admin",
		"password=pw",
	)
	stdout, _, err := runCLI(t, "--config", cfg, "--json", "fs", "get", "/no/such/file")
	if err == nil {
		t.Fatal("fs get on a missing path must fail")
	}
	if !strings.Contains(stdout, "synology_error") || !strings.Contains(stdout, "408") {
		t.Fatalf("expected synology_error 408 envelope, got %q", stdout)
	}
}

func TestPasswordStdinWorksWithConfigFilePassword(t *testing.T) {
	f := newFakeDSM(t)
	dir := t.TempDir()
	cfg := writeTestConfig(t, dir,
		"endpoint="+f.server.URL,
		"user=admin",
		"password=oldpassword",
	)
	var argsErr error
	stdout, _, argsErr := func() (string, string, error) {
		var out, errOut strings.Builder
		root, _ := newRootCmd(strings.NewReader("newpassword\n"), &out, &errOut)
		root.SetArgs([]string{"--config", cfg, "--json", "--password-stdin", "fs", "shares"})
		err := root.Execute()
		return out.String(), errOut.String(), err
	}()
	if argsErr != nil {
		t.Fatalf("--password-stdin with config password must not conflict: %v", argsErr)
	}
	if !strings.Contains(stdout, `"ok":true`) && !strings.Contains(stdout, `"ok": true`) {
		t.Fatalf("expected success, got %q", stdout)
	}
}

func TestCLIConfigInitForceRestoresSecureMode(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	if err := os.WriteFile(cfgPath, []byte("endpoint=https://old\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	_, _, err := runCLI(t, "--config", cfgPath, "cli-config", "init", "--force",
		"--endpoint", "https://nas:5001", "--user", "admin", "--password", "pw")
	if err != nil {
		t.Fatalf("cli-config init --force: %v", err)
	}
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("force overwrite left mode %04o, want 0600", perm)
	}
	content, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(content), "endpoint=https://nas:5001") {
		t.Fatalf("config not rewritten: %q", content)
	}
}

func TestCLIConfigInitRejectsInvalidEndpoint(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	_, _, err := runCLI(t, "--config", cfgPath, "cli-config", "init", "--endpoint", "not a url", "--user", "a", "--password", "b")
	if err == nil {
		t.Fatal("expected invalid endpoint to be rejected")
	}
}

func TestCLIConfigInitHonorsPasswordStdin(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	var out, errOut strings.Builder
	root, _ := newRootCmd(strings.NewReader("stdinpw\n"), &out, &errOut)
	root.SetArgs([]string{"--config", cfgPath, "cli-config", "init",
		"--endpoint", "https://nas:5001", "--user", "admin", "--password-stdin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("cli-config init --password-stdin: %v", err)
	}
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(content), "password=stdinpw") {
		t.Fatalf("stdin password not written to config: %q", content)
	}
}
