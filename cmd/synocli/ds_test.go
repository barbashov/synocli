package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"synocli/internal/synology/downloadstation"
)

func TestMapTaskIncludesStatusEnumFields(t *testing.T) {
	m := mapTask(downloadstation.Task{
		ID:     "dbid_1",
		Title:  "x",
		Status: "3",
	})
	if got := m["raw_status"]; got != "3" {
		t.Fatalf("raw_status=%v want 3", got)
	}
	if got := m["status_enum"]; got != "paused" {
		t.Fatalf("status_enum=%v want paused", got)
	}
	if got := m["status_display"]; got != "paused (3)" {
		t.Fatalf("status_display=%v want paused (3)", got)
	}
	code, ok := m["status_code"].(int)
	if !ok || code != 3 {
		t.Fatalf("status_code=%v want int(3)", m["status_code"])
	}
}

func TestWaitRejectsNonPositiveInterval(t *testing.T) {
	tests := []string{"0s", "-1s"}
	for _, interval := range tests {
		t.Run(interval, func(t *testing.T) {
			ac := &appContext{}
			cmd := newDSWaitCmd(ac)
			cmd.SetArgs([]string{"dbid_1", "--interval", interval})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "--interval must be greater than 0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWatchRejectsNonPositiveInterval(t *testing.T) {
	tests := []string{"0s", "-1s"}
	for _, interval := range tests {
		t.Run(interval, func(t *testing.T) {
			ac := &appContext{}
			cmd := newDSWatchCmd(ac)
			cmd.SetArgs([]string{"--interval", interval})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "--interval must be greater than 0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDetectAddInputKind(t *testing.T) {
	t.Run("magnet", func(t *testing.T) {
		kind, err := detectAddInputKind("magnet:?xt=urn:btih:abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != addInputMagnet {
			t.Fatalf("kind=%q want %q", kind, addInputMagnet)
		}
	})

	t.Run("url", func(t *testing.T) {
		kind, err := detectAddInputKind("https://example.com/file.iso")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != addInputURL {
			t.Fatalf("kind=%q want %q", kind, addInputURL)
		}
	})

	t.Run("torrent_file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "x.torrent")
		if err := os.WriteFile(path, []byte("not validated here"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		kind, err := detectAddInputKind(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != addInputTorrent {
			t.Fatalf("kind=%q want %q", kind, addInputTorrent)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := detectAddInputKind("no_scheme_or_file")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "cannot detect input type") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDSAddLegacySubcommandRejected(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
	cmd.SetArgs([]string{"ds", "add", "torrent", "https://example.com:5001", "./x.torrent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 3") {
		t.Fatalf("unexpected error: %v", err)
	}
}
