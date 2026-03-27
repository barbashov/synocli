package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIConfigShowRedactsPassword(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(cfg, []byte("endpoint=https://example.com:5001\nuser=admin\npassword=secret\ninsecure_tls=true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out, errOut bytes.Buffer
	cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
	cmd.SetArgs([]string{"--config", cfg, "cli-config", "show"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "secret") {
		t.Fatalf("password leaked in output: %q", got)
	}
	if !strings.Contains(got, "<redacted>") {
		t.Fatalf("expected redacted password in output: %q", got)
	}
}

func TestCLIConfigInitCreatesConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := filepath.Join(tmpDir, "config")
	var out, errOut bytes.Buffer
	cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
	cmd.SetArgs([]string{
		"--config", cfg,
		"--endpoint", "https://example.com:5001",
		"--user", "admin",
		"--password", "secret",
		"--insecure-tls",
		"cli-config", "init",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"endpoint=https://example.com:5001",
		"user=admin",
		"password=secret",
		"insecure_tls=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("config missing %q: %q", want, got)
		}
	}
	info, err := os.Stat(cfg)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode=%04o want 0600", info.Mode().Perm())
	}
}
