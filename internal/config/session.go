package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Session is the cached DSM session. Endpoint and User bind the SID to the
// identity it was issued for, so a cached token is never sent to a different
// host or silently reused for a different user.
type Session struct {
	SID      string `json:"sid"`
	Endpoint string `json:"endpoint"`
	User     string `json:"user"`
}

// SessionPathFromConfig derives the session file path from the directory
// that contains the config file.
func SessionPathFromConfig(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "session")
}

// LoadSession reads the session file. Returns a zero Session (no error) if the
// file does not exist, has permissions wider than 0600, or predates the bound
// JSON format; the latter two cases silently delete the file.
func LoadSession(path string) (Session, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Session{}, nil
		}
		return Session{}, fmt.Errorf("read session file: %w", err)
	}
	if mode := info.Mode().Perm(); PermTooOpen(mode) {
		_ = os.Remove(path)
		return Session{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Session{}, fmt.Errorf("read session file: %w", err)
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil || s.SID == "" || s.Endpoint == "" {
		// Legacy bare-SID file (or garbage): unusable because it is not bound
		// to an endpoint; drop it so the next login rewrites the new format.
		_ = os.Remove(path)
		return Session{}, nil
	}
	return s, nil
}

// WriteSession writes the session to the file with mode 0600.
func WriteSession(path string, s Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	b, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
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
