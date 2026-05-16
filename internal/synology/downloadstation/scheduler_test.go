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
const fixtureGetSchedulerBody = `{"data":{"download_rate":5000,"enable_schedule":true,"max_tasks":2,"max_tasks_limit":80,"order":"request","schedule":"221111111122222222222222221111111122222222222222221111111122222222222222221111111122222222222222221111111122222222222222221111111122222222222222221111111122222222222222","upload_rate":0},"success":true}`

func newSchedulerTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "test-sid", srv.Client(), "", 0, "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

func TestGetSchedulerConfigDecodesFixture(t *testing.T) {
	var capturedPath, capturedQuery, capturedCookie string
	c, _ := newSchedulerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			capturedCookie = ck.Value
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureGetSchedulerBody))
	})

	cfg, err := c.GetSchedulerConfig(context.Background())
	if err != nil {
		t.Fatalf("GetSchedulerConfig: %v", err)
	}

	if capturedPath != schedulerPath {
		t.Errorf("path = %q, want %q", capturedPath, schedulerPath)
	}
	for _, want := range []string{
		"api=SYNO.DownloadStation2.Settings.Scheduler",
		"version=1",
		"method=get",
		"_sid=test-sid",
	} {
		if !strings.Contains(capturedQuery, want) {
			t.Errorf("query missing %q: %s", want, capturedQuery)
		}
	}
	if capturedCookie != "test-sid" {
		t.Errorf("cookie id = %q, want test-sid", capturedCookie)
	}
	if cfg.MaxTasks != 2 {
		t.Errorf("MaxTasks = %d, want 2", cfg.MaxTasks)
	}
	if cfg.MaxTasksLimit != 80 {
		t.Errorf("MaxTasksLimit = %d, want 80", cfg.MaxTasksLimit)
	}
}

func TestSetSchedulerConfigSendsOnlyMaxTasks(t *testing.T) {
	var capturedQuery string
	c, _ := newSchedulerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	if err := c.SetSchedulerConfig(context.Background(), SchedulerConfigUpdate{MaxTasks: intPtr(3)}); err != nil {
		t.Fatalf("SetSchedulerConfig: %v", err)
	}
	for _, want := range []string{
		"api=SYNO.DownloadStation2.Settings.Scheduler",
		"version=1",
		"method=set",
		"_sid=test-sid",
		"max_tasks=3",
	} {
		if !strings.Contains(capturedQuery, want) {
			t.Errorf("query missing %q: %s", want, capturedQuery)
		}
	}
	// Regression: never send the sibling "Process order" radio. It is out of
	// scope for this command and would silently flip a setting we don't own.
	for _, forbidden := range []string{"order=", "download_rate=", "upload_rate=", "schedule=", "enable_schedule="} {
		if strings.Contains(capturedQuery, forbidden) {
			t.Errorf("query unexpectedly contains %q: %s", forbidden, capturedQuery)
		}
	}
}

func TestSetSchedulerConfigRejectsEmpty(t *testing.T) {
	c, _ := newSchedulerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called for empty update")
	})
	err := c.SetSchedulerConfig(context.Background(), SchedulerConfigUpdate{})
	if err == nil || !strings.Contains(err.Error(), "no fields set") {
		t.Fatalf("expected empty-update error, got %v", err)
	}
}

func TestGetSchedulerConfigPropagatesAPIError(t *testing.T) {
	c, _ := newSchedulerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":105}}`))
	})
	_, err := c.GetSchedulerConfig(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 105 {
		t.Errorf("code = %d, want 105", apiErr.Code)
	}
}

func TestSetSchedulerConfigPropagatesAPIError(t *testing.T) {
	c, _ := newSchedulerTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":101}}`))
	})
	err := c.SetSchedulerConfig(context.Background(), SchedulerConfigUpdate{MaxTasks: intPtr(3)})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 101 {
		t.Errorf("code = %d, want 101", apiErr.Code)
	}
}
