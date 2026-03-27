package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndLoadSession(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	sid := "abc123xyzSID"
	if err := WriteSession(p, sid); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got != sid {
		t.Fatalf("got SID %q, want %q", got, sid)
	}
}

func TestLoadSessionMissing(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession on missing file: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty SID, got %q", got)
	}
}

func TestLoadSessionBadPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	if err := os.WriteFile(p, []byte("somesid\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession with bad perms: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty SID for bad perms, got %q", got)
	}
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Fatal("expected bad-perms session file to be deleted")
	}
}

func TestDeleteSessionIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	if err := DeleteSession(p); err != nil {
		t.Fatalf("DeleteSession on missing file: %v", err)
	}
}

func TestSessionPathFromConfig(t *testing.T) {
	got := SessionPathFromConfig("/home/user/.synocli/config")
	want := "/home/user/.synocli/session"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteSessionCreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "newdir", "session")
	if err := WriteSession(p, "testsid"); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got != "testsid" {
		t.Fatalf("got %q, want %q", got, "testsid")
	}
}
