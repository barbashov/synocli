package redact

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestValueRedactsSecrets(t *testing.T) {
	if got := Value("password", "secret"); got != "<redacted>" {
		t.Fatalf("expected redacted, got %q", got)
	}
	if got := Value("normal", "value"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
}

func TestValueRedactsExpandedSecretKeys(t *testing.T) {
	for _, key := range []string{"secret", "api_key", "apikey", "passphrase", "key", "TOKEN"} {
		if got := Value(key, "x"); got != "<redacted>" {
			t.Errorf("Value(%q) = %q, want <redacted>", key, got)
		}
	}
}

func TestHeaderValueRedactsSensitiveHeaders(t *testing.T) {
	redacted := []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-SYNO-TOKEN", "X-Auth-Token"}
	for _, h := range redacted {
		if got := HeaderValue(h, "sensitive"); got != "<redacted>" {
			t.Errorf("HeaderValue(%q) = %q, want <redacted>", h, got)
		}
	}
	if got := HeaderValue("X-Test", "ok"); got != "ok" {
		t.Fatalf("expected unchanged, got %q", got)
	}
	if got := HeaderValue("Content-Type", "application/json"); got != "application/json" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestValueRedactsSuffixedSecretKeys(t *testing.T) {
	for _, key := range []string{"extract_password", "unlock_passphrase", "Extract_Password", "access_token", "client_secret"} {
		if got := Value(key, "x"); got != "<redacted>" {
			t.Errorf("Value(%q) = %q, want <redacted>", key, got)
		}
	}
	if got := Value("keyword", "x"); got != "x" {
		t.Fatalf("keyword must not be redacted, got %q", got)
	}
}

func TestURLStringRedactsQueryAndUserinfo(t *testing.T) {
	raw := "https://admin:supersecret@nas:5001/webapi/auth.cgi?account=admin&passwd=hunter2&method=login&_sid=abc"
	got := URLString(raw)
	for _, leak := range []string{"hunter2", "supersecret", "_sid=abc"} {
		if strings.Contains(got, leak) {
			t.Fatalf("URLString leaked %q: %q", leak, got)
		}
	}
	if !strings.Contains(got, "method=login") {
		t.Fatalf("non-secret query values should survive, got %q", got)
	}
}

func TestErrorRedactsURLError(t *testing.T) {
	cause := errors.New("connection refused")
	ue := &url.Error{Op: "Get", URL: "https://nas:5001/webapi/auth.cgi?passwd=hunter2&account=admin", Err: cause}
	got := Error(ue)
	if strings.Contains(got.Error(), "hunter2") {
		t.Fatalf("Error leaked password: %q", got.Error())
	}
	if !strings.Contains(got.Error(), "connection refused") {
		t.Fatalf("underlying cause must survive: %q", got.Error())
	}
	if !errors.Is(got, cause) {
		t.Fatal("underlying cause must stay reachable via errors.Is")
	}
}

func TestErrorPassesThroughNonURLErrors(t *testing.T) {
	plain := errors.New("plain")
	if got := Error(plain); got != plain {
		t.Fatalf("non-url errors must pass through unchanged, got %v", got)
	}
	if Error(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}
