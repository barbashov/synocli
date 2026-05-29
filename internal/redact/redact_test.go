package redact

import "testing"

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
