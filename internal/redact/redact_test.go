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

func TestHeaderValueRedactsSensitiveHeaders(t *testing.T) {
	if got := HeaderValue("Authorization", "Bearer abc"); got != "<redacted>" {
		t.Fatalf("expected redacted, got %q", got)
	}
	if got := HeaderValue("X-Test", "ok"); got != "ok" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}
