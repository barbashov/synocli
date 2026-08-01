package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	sid, err := c.Login(context.Background(), "user", "pass", "FileStation")
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
	if err := c.Logout(context.Background(), "xyz", "FileStation"); err != nil {
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
	_, err := c.Login(context.Background(), "u", "p", "DownloadStation")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, parseErr := url.Parse(ts.URL); parseErr != nil {
		t.Fatalf("unexpected parse error: %v", parseErr)
	}
}

func TestLoginTransportErrorDoesNotLeakPassword(t *testing.T) {
	// Point at a closed listener so http.Client.Do fails with a *url.Error
	// whose URL carries account/passwd query parameters.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := ts.URL
	ts.Close()

	c := &Client{Endpoint: endpoint, Path: "/webapi/auth.cgi", Version: 6, HTTP: &http.Client{}}
	_, err := c.Login(context.Background(), "admin", "hunter2secret", "synocli")
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), "hunter2secret") {
		t.Fatalf("login error leaked password: %v", err)
	}
	if strings.Contains(err.Error(), "account=admin") {
		t.Fatalf("login error leaked account: %v", err)
	}
}

func TestLogoutTransportErrorDoesNotLeakSID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := ts.URL
	ts.Close()

	c := &Client{Endpoint: endpoint, Path: "/webapi/auth.cgi", Version: 6, HTTP: &http.Client{}}
	err := c.Logout(context.Background(), "topsecretsid", "synocli")
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), "topsecretsid") {
		t.Fatalf("logout error leaked sid: %v", err)
	}
}
