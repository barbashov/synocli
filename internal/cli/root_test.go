package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/config"
)

func newRuntimeOptionsTestContext(t *testing.T) (*appContext, *cobra.Command) {
	t.Helper()
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config")
	if err := os.WriteFile(cfgPath, []byte(""), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ac := &appContext{
		opts: config.GlobalOptions{
			ConfigPath: cfgPath,
			Timeout:    30 * time.Second,
		},
		stdin: strings.NewReader(""),
	}
	cmd := &cobra.Command{Use: "test"}
	f := cmd.Flags()
	f.StringVar(&ac.opts.Endpoint, "endpoint", "", "")
	f.StringVar(&ac.opts.ConfigPath, "config", ac.opts.ConfigPath, "")
	f.StringVar(&ac.opts.User, "user", "", "")
	f.StringVar(&ac.opts.Password, "password", "", "")
	f.BoolVar(&ac.opts.PasswordStdin, "password-stdin", false, "")
	f.StringVar(&ac.opts.CredentialsFile, "credentials-file", "", "")
	f.BoolVar(&ac.opts.InsecureTLS, "insecure-tls", false, "")
	f.DurationVar(&ac.opts.Timeout, "timeout", 30*time.Second, "")
	f.BoolVar(&ac.opts.JSON, "json", false, "")
	f.BoolVar(&ac.opts.Debug, "debug", false, "")
	f.BoolVar(&ac.opts.NoUpdateCheck, "no-update-check", false, "")
	return ac, cmd
}

func TestResolveRuntimeOptionsPasswordStdinConflict(t *testing.T) {
	ac, cmd := newRuntimeOptionsTestContext(t)
	if err := cmd.ParseFlags([]string{"--password", "x", "--password-stdin"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if _, err := ac.resolveRuntimeOptions(cmd); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestResolveRuntimeOptionsCredentialsFileExclusive(t *testing.T) {
	ac, cmd := newRuntimeOptionsTestContext(t)
	if err := cmd.ParseFlags([]string{"--credentials-file", "/tmp/creds.env", "--password", "x"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if _, err := ac.resolveRuntimeOptions(cmd); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestResolveRuntimeOptionsCredentialsFileOverridesConfigUserPassword(t *testing.T) {
	ac, cmd := newRuntimeOptionsTestContext(t)
	if err := os.WriteFile(ac.opts.ConfigPath, []byte("user=admin\npassword=secret\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	credsPath := filepath.Join(filepath.Dir(ac.opts.ConfigPath), "creds")
	if err := os.WriteFile(credsPath, []byte("user=alice\npassword=fromfile\n"), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	if err := cmd.ParseFlags([]string{"--credentials-file", credsPath}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	got, err := ac.resolveRuntimeOptions(cmd)
	if err != nil {
		t.Fatalf("resolveRuntimeOptions: %v", err)
	}
	// The credentials file is authoritative and must override the config file's
	// user/password — and the username must be populated even on this path so
	// identity output is correct when a cached session skips login.
	if got.User != "alice" || got.Password != "fromfile" {
		t.Fatalf("expected creds-file values, got user=%q password=%q", got.User, got.Password)
	}
}

// Regression: with reuse_session enabled (cached-session path) and a
// credentials file, the username must still be resolved during option
// resolution so whoami/ping report the correct identity.
func TestResolveRuntimeOptionsLoadsUserWithReuseSession(t *testing.T) {
	ac, cmd := newRuntimeOptionsTestContext(t)
	if err := os.WriteFile(ac.opts.ConfigPath, []byte("reuse_session=true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	credsPath := filepath.Join(filepath.Dir(ac.opts.ConfigPath), "creds")
	if err := os.WriteFile(credsPath, []byte("user=alice\npassword=fromfile\n"), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	if err := cmd.ParseFlags([]string{"--credentials-file", credsPath}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	got, err := ac.resolveRuntimeOptions(cmd)
	if err != nil {
		t.Fatalf("resolveRuntimeOptions: %v", err)
	}
	if !got.ReuseSession {
		t.Fatal("ReuseSession should be true")
	}
	if got.User != "alice" {
		t.Fatalf("User not loaded from credentials file: got %q", got.User)
	}
}
