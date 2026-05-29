package filestation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Regression: a non-async search returns the WaitSearch snapshot directly, so it
// must request all results (limit=0) rather than capping at 1000.
func TestWaitSearchRequestsAllResults(t *testing.T) {
	var raw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true,"data":{"finished":true,"files":[]}}`))
	}))
	defer ts.Close()
	c, err := NewClient(ts.URL, "sidv", ts.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if _, err := c.WaitSearch(context.Background(), "task1", 10*time.Millisecond, time.Second); err != nil {
		t.Fatalf("WaitSearch error: %v", err)
	}
	if !strings.Contains(raw, "limit=0") {
		t.Fatalf("query %q must request all results (limit=0)", raw)
	}
	if strings.Contains(raw, "limit=1000") {
		t.Fatalf("query %q must not cap results at 1000", raw)
	}
}
