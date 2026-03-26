package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewEnvelope(t *testing.T) {
	start := time.Now().Add(-100 * time.Millisecond)
	env := NewEnvelope(true, "ds list", "https://nas:5001", start)
	if !env.OK {
		t.Fatal("expected OK=true")
	}
	if env.Command != "ds list" {
		t.Fatalf("Command=%q want %q", env.Command, "ds list")
	}
	if env.Meta.Endpoint != "https://nas:5001" {
		t.Fatalf("Endpoint=%q", env.Meta.Endpoint)
	}
	if env.Meta.DurationMS < 0 {
		t.Fatalf("DurationMS=%d should be >= 0", env.Meta.DurationMS)
	}
	if env.Meta.Timestamp == "" {
		t.Fatal("Timestamp should not be empty")
	}
	if _, err := time.Parse(time.RFC3339, env.Meta.Timestamp); err != nil {
		t.Fatalf("Timestamp is not RFC3339: %v", err)
	}
}

func TestNewEnvelopeError(t *testing.T) {
	env := NewEnvelope(false, "ds get", "", time.Now())
	if env.OK {
		t.Fatal("expected OK=false")
	}
}

func TestWriteJSON(t *testing.T) {
	env := NewEnvelope(true, "test", "", time.Now())
	env.Data = map[string]any{"key": "value"}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, env); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "\"ok\": true") {
		t.Fatalf("expected pretty-printed JSON, got: %s", got)
	}
	var decoded Envelope
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if !decoded.OK || decoded.Command != "test" {
		t.Fatalf("decoded envelope mismatch: %+v", decoded)
	}
}

func TestWriteJSONLine(t *testing.T) {
	env := NewEnvelope(true, "watch", "", time.Now())
	env.Data = map[string]any{"event": "snapshot"}
	var buf bytes.Buffer
	if err := WriteJSONLine(&buf, env); err != nil {
		t.Fatalf("WriteJSONLine: %v", err)
	}
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Fatal("expected trailing newline")
	}
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("expected single line, got %d newlines", strings.Count(got, "\n"))
	}
	var decoded Envelope
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestWriteJSONWithError(t *testing.T) {
	env := NewEnvelope(false, "ds get", "https://nas:5001", time.Now())
	env.Error = &ErrInfo{
		Code:    "not_found",
		Message: "task not found",
		Details: map[string]any{"synology_code": 404},
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, env); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var decoded Envelope
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Error == nil {
		t.Fatal("expected error field")
	}
	if decoded.Error.Code != "not_found" {
		t.Fatalf("error code=%q want not_found", decoded.Error.Code)
	}
}
