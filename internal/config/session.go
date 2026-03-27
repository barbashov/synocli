package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SessionPathFromConfig derives the session file path from the directory
// that contains the config file.
func SessionPathFromConfig(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "session")
}

// LoadSession reads the session file and returns the stored SID.
// Returns ("", nil) if the file does not exist or has permissions wider than
// 0600 (the latter case silently deletes the file).
func LoadSession(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read session file: %w", err)
	}
	if mode := info.Mode().Perm(); mode&0077 != 0 {
		_ = os.Remove(path)
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read session file: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

// WriteSession writes the SID to the session file with mode 0600.
func WriteSession(path string, sid string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(sid+"\n"), 0o600); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

// DeleteSession removes the session file. Returns nil if the file does not
// exist.
func DeleteSession(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete session file: %w", err)
	}
	return nil
}
