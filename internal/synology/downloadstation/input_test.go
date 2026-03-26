package downloadstation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectAddInputKind(t *testing.T) {
	t.Run("magnet", func(t *testing.T) {
		kind, err := DetectAddInputKind("magnet:?xt=urn:btih:abc")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != AddInputMagnet {
			t.Fatalf("kind=%q want %q", kind, AddInputMagnet)
		}
	})

	t.Run("url", func(t *testing.T) {
		kind, err := DetectAddInputKind("https://example.com/file.iso")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != AddInputURL {
			t.Fatalf("kind=%q want %q", kind, AddInputURL)
		}
	})

	t.Run("torrent_file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "x.torrent")
		if err := os.WriteFile(path, []byte("not validated here"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		kind, err := DetectAddInputKind(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != AddInputTorrent {
			t.Fatalf("kind=%q want %q", kind, AddInputTorrent)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		_, err := DetectAddInputKind("no_scheme_or_file")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "cannot detect input type") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
