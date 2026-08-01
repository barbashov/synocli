package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndLoadSession(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	want := Session{SID: "abc123xyzSID", Endpoint: "https://nas:5001", User: "admin"}
	if err := WriteSession(p, want); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got != want {
		t.Fatalf("got session %+v, want %+v", got, want)
	}
}

func TestLoadSessionMissing(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession on missing file: %v", err)
	}
	if got != (Session{}) {
		t.Fatalf("expected empty session, got %+v", got)
	}
}

func TestLoadSessionBadPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	if err := os.WriteFile(p, []byte(`{"sid":"somesid","endpoint":"https://nas:5001"}`), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession with bad perms: %v", err)
	}
	if got != (Session{}) {
		t.Fatalf("expected empty session for bad perms, got %+v", got)
	}
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Fatal("expected bad-perms session file to be deleted")
	}
}

func TestLoadSessionLegacyBareSID(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	if err := os.WriteFile(p, []byte("legacybareSID\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession on legacy file: %v", err)
	}
	if got != (Session{}) {
		t.Fatalf("expected legacy unbound session to be discarded, got %+v", got)
	}
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Fatal("expected legacy session file to be deleted")
	}
}

func TestLoadSessionMissingEndpointDiscarded(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "session")
	if err := os.WriteFile(p, []byte(`{"sid":"somesid"}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got != (Session{}) {
		t.Fatalf("expected session without endpoint to be discarded, got %+v", got)
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
	want := Session{SID: "testsid", Endpoint: "https://nas:5001", User: "admin"}
	if err := WriteSession(p, want); err != nil {
		t.Fatalf("WriteSession: %v", err)
	}
	got, err := LoadSession(p)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
