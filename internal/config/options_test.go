package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateEndpoint(t *testing.T) {
	if _, err := ValidateEndpoint("https://192.168.0.1:5001"); err != nil {
		t.Fatalf("unexpected endpoint error: %v", err)
	}
	if _, err := ValidateEndpoint("192.168.0.1"); err == nil {
		t.Fatal("expected error for missing scheme")
	}
}

func TestResolvePasswordFromStdin(t *testing.T) {
	opts := GlobalOptions{User: "admin", PasswordStdin: true}
	if err := opts.ResolvePassword(strings.NewReader("secret\n")); err != nil {
		t.Fatalf("ResolvePassword error: %v", err)
	}
	if opts.Password != "secret" {
		t.Fatalf("unexpected password %q", opts.Password)
	}
}

func TestResolvePasswordFromCredentialsFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "creds.env")
	content := strings.Join([]string{
		"# comment",
		"User = admin",
		"PASSWORD=secret",
		"ignored_key=ignored",
		"",
	}, "\n")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	opts := GlobalOptions{CredentialsFile: p}
	if err := opts.ResolvePassword(strings.NewReader("")); err != nil {
		t.Fatalf("ResolvePassword error: %v", err)
	}
	if opts.User != "admin" {
		t.Fatalf("unexpected user %q", opts.User)
	}
	if opts.Password != "secret" {
		t.Fatalf("unexpected password %q", opts.Password)
	}
}

func TestResolvePasswordFromCredentialsFileMissingPassword(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "creds.env")
	if err := os.WriteFile(p, []byte("user=admin\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	opts := GlobalOptions{CredentialsFile: p}
	if err := opts.ResolvePassword(strings.NewReader("")); err == nil {
		t.Fatal("expected error for missing password")
	}
}

func TestResolvePasswordCredentialsFileWithFlagsConflict(t *testing.T) {
	opts := GlobalOptions{CredentialsFile: "/tmp/creds.env", User: "admin"}
	if err := opts.ResolvePassword(strings.NewReader("")); err == nil {
		t.Fatal("expected conflict error")
	}
}
