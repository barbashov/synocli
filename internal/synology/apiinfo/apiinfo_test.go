package apiinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscover(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"SYNO.API.Auth":{"path":"auth.cgi","minVersion":1,"maxVersion":6}}}`))
	}))
	defer ts.Close()
	entries, err := Discover(context.Background(), ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if entries["SYNO.API.Auth"].MaxVersion != 6 {
		t.Fatalf("unexpected max version: %+v", entries["SYNO.API.Auth"])
	}
}

func TestSelect(t *testing.T) {
	entries := map[string]Entry{"A": {Path: "p.cgi", MaxVersion: 3}}
	path, version := Select(entries, "A", "/fallback", 1)
	if path != "/webapi/p.cgi" || version != 3 {
		t.Fatalf("unexpected selection %s %d", path, version)
	}
	path, version = Select(entries, "B", "/fallback", 1)
	if path != "/fallback" || version != 1 {
		t.Fatalf("unexpected fallback %s %d", path, version)
	}
}
