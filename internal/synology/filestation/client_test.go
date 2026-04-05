package filestation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientValidation(t *testing.T) {
	_, err := NewClient("", "sid", &http.Client{}, nil)
	if err == nil || !strings.Contains(err.Error(), "endpoint is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = NewClient("https://example.com", "", &http.Client{}, nil)
	if err == nil || !strings.Contains(err.Error(), "sid is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = NewClient("https://example.com", "sid", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "http client is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallBuildsExpectedQuery(t *testing.T) {
	var raw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true,"data":{"ok":true}}`))
	}))
	defer ts.Close()
	c, err := NewClient(ts.URL, "sidv", ts.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	var out map[string]any
	if err := c.Call(context.Background(), APIInfo, "get", nil, &out); err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("unexpected output: %#v", out)
	}
	for _, want := range []string{"api=SYNO.FileStation.Info", "method=get", "version=2", "_sid=sidv"} {
		if !strings.Contains(raw, want) {
			t.Fatalf("query %q missing %q", raw, want)
		}
	}
}

func TestCallDecodesArraySubError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":1002,"errors":[{"code":408,"path":"/Data/missing"}]}}`))
	}))
	defer ts.Close()
	c, err := NewClient(ts.URL, "sidv", ts.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	err = c.Call(context.Background(), APICopyMove, "start", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("unexpected error type: %T", err)
	}
	if apiErr.Code != 1002 || apiErr.SubCode != 408 || apiErr.Path != "/Data/missing" {
		t.Fatalf("unexpected api error: %#v", apiErr)
	}
}

func TestCallDecodesObjectSubError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":408,"errors":{"name":"not_found"}}}`))
	}))
	defer ts.Close()
	c, err := NewClient(ts.URL, "sidv", ts.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	err = c.Call(context.Background(), APIList, "list", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("unexpected error type: %T", err)
	}
	if apiErr.Code != 408 {
		t.Fatalf("unexpected code: %#v", apiErr)
	}
}

func TestErrorMessage(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{408, "file or folder does not exist"},
		{418, "invalid path format"},
		{999, "unmapped"},
	}
	for _, tc := range cases {
		if got := ErrorMessage(tc.code); got != tc.want {
			t.Errorf("ErrorMessage(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}
