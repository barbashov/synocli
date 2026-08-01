package httpclient

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewUsesProxyAwareTransport(t *testing.T) {
	c, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", c.Transport)
	}
	if tr.Proxy == nil {
		t.Fatal("transport must honor proxy environment variables")
	}
	if tr.TLSHandshakeTimeout == 0 {
		t.Fatal("transport must keep DefaultTransport handshake timeout")
	}
}

func TestDebugTransportStreamsLargeResponseBody(t *testing.T) {
	payload := make([]byte, maxBodyCapture*3)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	defer ts.Close()

	var debug bytes.Buffer
	c, err := New(Options{Debug: true, DebugOut: &debug, Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("body corrupted by debug transport: got %d bytes, want %d", len(got), len(payload))
	}
	if !strings.Contains(debug.String(), "<omitted: larger than") {
		t.Fatalf("expected omission notice for large body, got %q", debug.String())
	}
}

func TestDebugTransportLogsSmallJSONBodyRedacted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"sid":"secret-sid"}}`))
	}))
	defer ts.Close()

	var debug bytes.Buffer
	c, err := New(Options{Debug: true, DebugOut: &debug})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	resp, err := c.Get(ts.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "secret-sid") {
		t.Fatal("caller must still see the raw body")
	}
	out := debug.String()
	if strings.Contains(out, "secret-sid") {
		t.Fatalf("debug output leaked sid: %q", out)
	}
	if !strings.Contains(out, "redacted") {
		t.Fatalf("expected redacted sid in debug output, got %q", out)
	}
}

type failingRT struct{ err error }

func (f failingRT) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }

func TestDebugTransportRedactsTransportErrors(t *testing.T) {
	var debug bytes.Buffer
	ue := &url.Error{
		Op:  "Get",
		URL: "https://nas:5001/webapi/auth.cgi?account=admin&passwd=hunter2",
		Err: errors.New("tls: failed to verify certificate"),
	}
	rt := &debugRoundTripper{next: failingRT{err: ue}, out: &debug}
	req, _ := http.NewRequest(http.MethodGet, "https://nas:5001/webapi/auth.cgi?account=admin&passwd=hunter2", nil)
	if _, err := rt.RoundTrip(req); err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(debug.String(), "hunter2") {
		t.Fatalf("debug error line leaked password: %q", debug.String())
	}
}
