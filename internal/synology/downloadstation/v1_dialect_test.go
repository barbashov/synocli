package downloadstation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newV1Client(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	c, err := NewClient(ts.URL, "sid1", ts.Client(), "/webapi/DownloadStation/task.cgi", 1, "SYNO.DownloadStation.Task")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestV1GetUsesGetinfoAndCommaEncoding(t *testing.T) {
	c := newV1Client(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("method"); got != "getinfo" {
			t.Errorf("v1 method = %q, want getinfo", got)
		}
		if got := q.Get("id"); got != "dbid_1" {
			t.Errorf("v1 id = %q, want plain dbid_1", got)
		}
		if add := q.Get("additional"); strings.HasPrefix(add, "[") {
			t.Errorf("v1 additional must be comma-separated, got %q", add)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"tasks": []any{map[string]any{"id": "dbid_1", "title": "t", "status": "downloading"}}},
		})
	})
	task, err := c.Get(context.Background(), "dbid_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.ID != "dbid_1" {
		t.Fatalf("task id = %q", task.ID)
	}
}

func TestV1ListUsesCommaEncoding(t *testing.T) {
	c := newV1Client(t, func(w http.ResponseWriter, r *http.Request) {
		if add := r.URL.Query().Get("additional"); strings.HasPrefix(add, "[") {
			t.Errorf("v1 additional must be comma-separated, got %q", add)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"data":    map[string]any{"tasks": []any{}},
		})
	})
	if _, err := c.List(context.Background()); err != nil {
		t.Fatalf("List: %v", err)
	}
}

func TestV1AddURIUsesURIParameter(t *testing.T) {
	c := newV1Client(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("uri"); got != "http://example.com/file.iso" {
			t.Errorf("v1 create uri = %q", got)
		}
		if q.Get("url") != "" || q.Get("type") != "" || q.Get("create_list") != "" {
			t.Errorf("v1 create must not send DS2 parameters, got %v", q)
		}
		// The v1 create method returns bare success without task ids.
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	})
	ids, err := c.AddURI(context.Background(), "http://example.com/file.iso", "downloads")
	if err != nil {
		t.Fatalf("AddURI: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("v1 create returns no ids, got %v", ids)
	}
}

func TestDecodeActionV1ArraySuccess(t *testing.T) {
	body := `{"success":true,"data":[{"id":"dbid_1","error":0},{"id":"dbid_2","error":0}]}`
	if err := decodeAction(strings.NewReader(body)); err != nil {
		t.Fatalf("decodeAction v1 success: %v", err)
	}
}

func TestDecodeActionV1ArrayPerTaskError(t *testing.T) {
	body := `{"success":true,"data":[{"id":"dbid_1","error":0},{"id":"dbid_2","error":405}]}`
	err := decodeAction(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected per-task error to surface")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 405 || len(apiErr.FailedTasks) != 1 || apiErr.FailedTasks[0].ID != "dbid_2" {
		t.Fatalf("unexpected APIError: %+v", apiErr)
	}
}

func TestDecodeActionDS2ObjectStillWorks(t *testing.T) {
	if err := decodeAction(strings.NewReader(`{"success":true,"data":{"failed_task":[]}}`)); err != nil {
		t.Fatalf("DS2 empty failed_task: %v", err)
	}
	err := decodeAction(strings.NewReader(`{"success":true,"data":{"failed_task":[{"id":"dbid_9","error":544}]}}`))
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != 544 {
		t.Fatalf("DS2 failed_task not surfaced: %v", err)
	}
}
