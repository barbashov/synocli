package downloadstation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTorrentFile(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		path := writeTorrentFixture(t, []byte("d4:infod6:lengthi1e4:name1:x12:piece lengthi16384e6:pieces20:12345678901234567890ee"))
		if err := ValidateTorrentFile(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("malformed_bencode", func(t *testing.T) {
		path := writeTorrentFixture(t, []byte("d4:info"))
		err := ValidateTorrentFile(path)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invalid bencode") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing_info", func(t *testing.T) {
		path := writeTorrentFixture(t, []byte("d8:announce3:abce"))
		err := ValidateTorrentFile(path)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "missing info dictionary") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("root_not_dict", func(t *testing.T) {
		path := writeTorrentFixture(t, []byte("l4:infoe"))
		err := ValidateTorrentFile(path)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "top-level bencode value must be dictionary") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func writeTorrentFixture(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.torrent")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}
