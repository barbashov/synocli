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
	if err := cmd.ParseFlags([]string{"--credentials-file", "/tmp/creds.env"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	got, err := ac.resolveRuntimeOptions(cmd)
	if err != nil {
		t.Fatalf("resolveRuntimeOptions: %v", err)
	}
	if got.User != "" || got.Password != "" {
		t.Fatalf("expected user/password to be cleared, got user=%q password=%q", got.User, got.Password)
	}
}
