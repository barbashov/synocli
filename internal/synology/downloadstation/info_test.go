package downloadstation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Captured verbatim from a live DSM 7 NAS via the Step-0 probe.
const fixtureGetConfigBody = `{"data":{"bt_max_download":5000,"bt_max_upload":0,"default_destination":"transmission/download","emule_default_destination":null,"emule_enabled":false,"emule_max_download":0,"emule_max_upload":20,"ftp_max_download":0,"http_max_download":0,"nzb_max_download":0,"unzip_service_enabled":false},"success":true}`

func newInfoTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "test-sid", srv.Client(), "", 0, "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

func TestGetServerConfigDecodesFixture(t *testing.T) {
	var capturedPath, capturedQuery, capturedCookie string
	c, _ := newInfoTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			capturedCookie = ck.Value
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureGetConfigBody))
	})

	cfg, err := c.GetServerConfig(context.Background())
	if err != nil {
		t.Fatalf("GetServerConfig: %v", err)
	}

	if capturedPath != infoPath {
		t.Errorf("path = %q, want %q", capturedPath, infoPath)
	}
	for _, want := range []string{
		"api=SYNO.DownloadStation.Info",
		"version=2",
		"method=getconfig",
		"_sid=test-sid",
	} {
		if !strings.Contains(capturedQuery, want) {
			t.Errorf("query missing %q: %s", want, capturedQuery)
		}
	}
	if capturedCookie != "test-sid" {
		t.Errorf("cookie id = %q, want test-sid", capturedCookie)
	}
	if cfg.BTMaxDownload != 5000 {
		t.Errorf("BTMaxDownload = %d, want 5000", cfg.BTMaxDownload)
	}
	if cfg.BTMaxUpload != 0 {
		t.Errorf("BTMaxUpload = %d, want 0", cfg.BTMaxUpload)
	}
	if cfg.EmuleMaxUpload != 20 {
		t.Errorf("EmuleMaxUpload = %d, want 20", cfg.EmuleMaxUpload)
	}
	if cfg.DefaultDestination != "transmission/download" {
		t.Errorf("DefaultDestination = %q", cfg.DefaultDestination)
	}
	if cfg.EmuleDefaultDestination != "" {
		t.Errorf("EmuleDefaultDestination = %q, want empty (null in JSON)", cfg.EmuleDefaultDestination)
	}
	if cfg.EmuleEnabled || cfg.UnzipServiceEnabled {
		t.Errorf("expected emule_enabled=false, unzip_service_enabled=false")
	}
}

func TestSetServerConfigSendsOnlySetFields(t *testing.T) {
	tests := []struct {
		name        string
		update      ServerConfigUpdate
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:        "only download",
			update:      ServerConfigUpdate{BTMaxDownload: intPtr(1024)},
			wantPresent: []string{"bt_max_download=1024"},
			wantAbsent:  []string{"bt_max_upload="},
		},
		{
			name:        "only upload",
			update:      ServerConfigUpdate{BTMaxUpload: intPtr(0)},
			wantPresent: []string{"bt_max_upload=0"},
			wantAbsent:  []string{"bt_max_download="},
		},
		{
			name:        "both",
			update:      ServerConfigUpdate{BTMaxDownload: intPtr(2048), BTMaxUpload: intPtr(256)},
			wantPresent: []string{"bt_max_download=2048", "bt_max_upload=256"},
			wantAbsent:  nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedQuery string
			c, _ := newInfoTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.RawQuery
				_, _ = w.Write([]byte(`{"success":true}`))
			})
			if err := c.SetServerConfig(context.Background(), tc.update); err != nil {
				t.Fatalf("SetServerConfig: %v", err)
			}
			for _, want := range append([]string{
				"api=SYNO.DownloadStation.Info",
				"version=2",
				"method=setserverconfig",
				"_sid=test-sid",
			}, tc.wantPresent...) {
				if !strings.Contains(capturedQuery, want) {
					t.Errorf("query missing %q: %s", want, capturedQuery)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(capturedQuery, absent) {
					t.Errorf("query unexpectedly contains %q: %s", absent, capturedQuery)
				}
			}
		})
	}
}

func TestSetServerConfigRejectsEmpty(t *testing.T) {
	c, _ := newInfoTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called for empty update")
	})
	err := c.SetServerConfig(context.Background(), ServerConfigUpdate{})
	if err == nil || !strings.Contains(err.Error(), "no fields set") {
		t.Fatalf("expected empty-update error, got %v", err)
	}
}

func TestGetServerConfigPropagatesAPIError(t *testing.T) {
	c, _ := newInfoTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":105}}`))
	})
	_, err := c.GetServerConfig(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 105 {
		t.Errorf("code = %d, want 105", apiErr.Code)
	}
}

func TestSetServerConfigPropagatesAPIError(t *testing.T) {
	c, _ := newInfoTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":101}}`))
	})
	err := c.SetServerConfig(context.Background(), ServerConfigUpdate{BTMaxDownload: intPtr(1024)})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 101 {
		t.Errorf("code = %d, want 101", apiErr.Code)
	}
}

func intPtr(v int) *int { return &v }
