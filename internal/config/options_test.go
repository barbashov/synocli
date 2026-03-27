package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestParseConfigFile(t *testing.T) {
	got, err := ParseConfigFile(strings.Join([]string{
		"endpoint=https://example.com:5001",
		"user=admin",
		"password=secret",
		"insecure_tls=true",
		"timeout=45s",
	}, "\n"))
	if err != nil {
		t.Fatalf("ParseConfigFile: %v", err)
	}
	if got.Endpoint != "https://example.com:5001" {
		t.Fatalf("endpoint=%q", got.Endpoint)
	}
	if got.User != "admin" || got.Password != "secret" {
		t.Fatalf("unexpected credentials: user=%q password=%q", got.User, got.Password)
	}
	if !got.InsecureTLS {
		t.Fatal("expected insecure_tls=true")
	}
	if got.Timeout != 45*time.Second {
		t.Fatalf("timeout=%s", got.Timeout)
	}
}

func TestParseConfigFileReuseSession(t *testing.T) {
	got, err := ParseConfigFile("reuse_session=true\n")
	if err != nil {
		t.Fatalf("ParseConfigFile: %v", err)
	}
	if !got.ReuseSession {
		t.Fatal("expected ReuseSession=true")
	}

	got2, err := ParseConfigFile("reuse_session=false\n")
	if err != nil {
		t.Fatalf("ParseConfigFile false: %v", err)
	}
	if got2.ReuseSession {
		t.Fatal("expected ReuseSession=false")
	}

	if _, err := ParseConfigFile("reuse_session=notbool\n"); err == nil {
		t.Fatal("expected parse error for invalid bool")
	}
}

func TestLoadConfigFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(p, []byte("endpoint=https://example.com:5001\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := LoadConfigFile(p, true); err == nil {
		t.Fatal("expected permission error")
	}
}
