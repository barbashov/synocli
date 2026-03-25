package output

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type Envelope struct {
	OK      bool     `json:"ok"`
	Command string   `json:"command"`
	Data    any      `json:"data,omitempty"`
	Error   *ErrInfo `json:"error,omitempty"`
	Meta    MetaInfo `json:"meta"`
}

type ErrInfo struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type MetaInfo struct {
	Timestamp  string         `json:"timestamp"`
	DurationMS int64          `json:"duration_ms"`
	Endpoint   string         `json:"endpoint"`
	APIVersion map[string]int `json:"api_version,omitempty"`
}

func NewEnvelope(ok bool, command, endpoint string, start time.Time) Envelope {
	return Envelope{
		OK:      ok,
		Command: command,
		Meta: MetaInfo{
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			DurationMS: time.Since(start).Milliseconds(),
			Endpoint:   endpoint,
		},
	}
}

func WriteJSON(w io.Writer, env Envelope) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

func WriteJSONLine(w io.Writer, env Envelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
