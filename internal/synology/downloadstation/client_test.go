package downloadstation

import (
	"net/http"
	"strings"
	"testing"
)

func TestNewClientValidation(t *testing.T) {
	_, err := NewClient("", "sid", &http.Client{}, "", 0, "")
	if err == nil || !strings.Contains(err.Error(), "endpoint is required") {
		t.Fatalf("unexpected endpoint validation error: %v", err)
	}

	_, err = NewClient("https://example.com", "", &http.Client{}, "", 0, "")
	if err == nil || !strings.Contains(err.Error(), "sid is required") {
		t.Fatalf("unexpected sid validation error: %v", err)
	}

	_, err = NewClient("https://example.com", "sid", nil, "", 0, "")
	if err == nil || !strings.Contains(err.Error(), "http client is required") {
		t.Fatalf("unexpected http validation error: %v", err)
	}
}

func TestNewClientDefaults(t *testing.T) {
	hc := &http.Client{}
	c, err := NewClient("https://example.com", "sidv", hc, "", 0, "")
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	if c.endpoint != "https://example.com" {
		t.Fatalf("unexpected endpoint: %q", c.endpoint)
	}
	if c.sid != "sidv" {
		t.Fatalf("unexpected sid: %q", c.sid)
	}
	if c.http != hc {
		t.Fatalf("unexpected http client pointer")
	}
	if c.path != defaultPath {
		t.Fatalf("unexpected default path: %q", c.path)
	}
	if c.version != defaultVersion {
		t.Fatalf("unexpected default version: %d", c.version)
	}
	if c.apiName != defaultAPIName {
		t.Fatalf("unexpected default api name: %q", c.apiName)
	}
}
