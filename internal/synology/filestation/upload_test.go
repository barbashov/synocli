package filestation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newUploadTestServer serves just enough of the FileStation API for
// UploadRecursiveCP to reach the tree walk: getinfo says nothing exists and
// create always succeeds.
func newUploadTestServer(t *testing.T) (*Client, *int) {
	t.Helper()
	uploads := new(int)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			*uploads++
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{}})
			return
		}
		switch r.URL.Query().Get("method") {
		case "getinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data":    map[string]any{"files": []any{map[string]any{"code": 408}}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		}
	}))
	t.Cleanup(ts.Close)
	c, err := NewClient(ts.URL, "sid1", ts.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, uploads
}

func TestUploadRecursiveRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks not reliable on windows CI")
	}
	c, uploads := newUploadTestServer(t)

	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside-content"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "normal.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write normal: %v", err)
	}
	if err := os.Symlink(secret, filepath.Join(tree, "zz-link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, err := c.UploadRecursiveCP(context.Background(), tree, "/dest", true, false, false)
	if err == nil {
		t.Fatal("expected symlink in tree to be rejected")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = uploads
}
