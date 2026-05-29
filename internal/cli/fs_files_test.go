package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// errReader yields some bytes, then fails — simulating a connection dropped
// mid-download.
type errReader struct {
	data []byte
	pos  int
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, errors.New("connection reset")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestStreamToFileAtomicSuccess(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "result.bin")
	want := []byte("hello world payload")
	n, err := streamToFileAtomic(out, bytes.NewReader(want))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != int64(len(want)) {
		t.Fatalf("got %d bytes, want %d", n, len(want))
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Regression: a mid-stream failure must not truncate or destroy an existing
// file at the output path, nor leave a partial file behind.
func TestStreamToFileAtomicPreservesExistingOnFailure(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "result.bin")
	original := []byte("ORIGINAL CONTENTS — MUST SURVIVE")
	if err := os.WriteFile(out, original, 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	_, err := streamToFileAtomic(out, &errReader{data: []byte("partial...")})
	if err == nil {
		t.Fatal("expected error from failed copy, got nil")
	}

	got, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("existing file was modified: got %q, want %q", got, original)
	}

	// No leftover temp files in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "result.bin" {
			t.Fatalf("unexpected leftover file: %s", e.Name())
		}
	}
}

var _ io.Reader = (*errReader)(nil)
