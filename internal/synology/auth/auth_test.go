package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestLoginRequestAndResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("api") != "SYNO.API.Auth" || q.Get("method") != "login" || q.Get("account") != "user" || q.Get("passwd") != "pass" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"sid":"abc"}}`))
	}))
	defer ts.Close()
	c := &Client{Endpoint: ts.URL, Path: "/auth", Version: 6, HTTP: ts.Client()}
	sid, err := c.Login(context.Background(), "user", "pass")
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if sid != "abc" {
		t.Fatalf("unexpected sid %q", sid)
	}
}

func TestLogoutIncludesSID(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Query().Get("_sid") != "xyz" {
			t.Fatalf("missing _sid in query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()
	c := &Client{Endpoint: ts.URL, Path: "/auth", Version: 6, HTTP: ts.Client()}
	if err := c.Logout(context.Background(), "xyz"); err != nil {
		t.Fatalf("Logout error: %v", err)
	}
	if !called {
		t.Fatal("logout endpoint not called")
	}
}

func TestLoginFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":400}}`))
	}))
	defer ts.Close()
	c := &Client{Endpoint: ts.URL, Path: "/auth", Version: 6, HTTP: ts.Client()}
	_, err := c.Login(context.Background(), "u", "p")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, parseErr := url.Parse(ts.URL); parseErr != nil {
		t.Fatalf("unexpected parse error: %v", parseErr)
	}
}
