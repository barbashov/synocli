package downloadstation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustNewClient(t *testing.T, endpoint, sid string, httpClient *http.Client, path string, version int, apiName string) *Client {
	t.Helper()
	c, err := NewClient(endpoint, sid, httpClient, path, version, apiName)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	return c
}

func newV1TestClient(t *testing.T, endpoint, sid string, httpClient *http.Client) *Client {
	t.Helper()
	return mustNewClient(t, endpoint, sid, httpClient, "/task", 1, "")
}

func newDS2TestClient(t *testing.T, endpoint, sid string, httpClient *http.Client) *Client {
	t.Helper()
	return mustNewClient(t, endpoint, sid, httpClient, "/webapi/entry.cgi", 2, "SYNO.DownloadStation2.Task")
}

func TestPauseBuildsExpectedQuery(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()
	c := newV1TestClient(t, ts.URL, "sid123", ts.Client())
	if err := c.Pause(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatalf("Pause error: %v", err)
	}
	if !strings.Contains(rawQuery, "method=pause") || !strings.Contains(rawQuery, "id=a%2Cb") || !strings.Contains(rawQuery, "_sid=sid123") {
		t.Fatalf("unexpected query: %s", rawQuery)
	}
}

func TestDeleteBuildsExpectedQueryWithoutForceComplete(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()
	c := newV1TestClient(t, ts.URL, "sid123", ts.Client())
	if err := c.Delete(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if !strings.Contains(rawQuery, "method=delete") || !strings.Contains(rawQuery, "id=a%2Cb") || !strings.Contains(rawQuery, "_sid=sid123") {
		t.Fatalf("unexpected query: %s", rawQuery)
	}
	if strings.Contains(rawQuery, "force_complete=") {
		t.Fatalf("force_complete must not be set: %s", rawQuery)
	}
}

func TestListIncludesUnlimitedPaging(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true,"data":{"tasks":[]}}`))
	}))
	defer ts.Close()
	c := newV1TestClient(t, ts.URL, "sid123", ts.Client())
	if _, err := c.List(context.Background()); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if !strings.Contains(rawQuery, "offset=0") || !strings.Contains(rawQuery, "limit=-1") {
		t.Fatalf("expected unlimited paging in query, got: %s", rawQuery)
	}
}

func TestDS2AddTorrentRequiresTaskID(t *testing.T) {
	var requestPath string
	var fields map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		fields = map[string]string{}
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader error: %v", err)
		}
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			if part.FileName() != "" {
				continue
			}
			b := make([]byte, 4096)
			n, _ := part.Read(b)
			fields[part.FormName()] = string(b[:n])
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"list_id":["abc"],"task_id":[]}}`))
	}))
	defer ts.Close()
	tmpDir := t.TempDir()
	torrentPath := filepath.Join(tmpDir, "x.torrent")
	if err := os.WriteFile(torrentPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write torrent file: %v", err)
	}
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	_, err := c.AddTorrent(context.Background(), torrentPath, "downloads")
	if err == nil {
		t.Fatalf("expected error when task_id is empty")
	}
	if !strings.Contains(err.Error(), "list_id without task_id") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestPath != "/webapi/entry.cgi/SYNO.DownloadStation2.Task" {
		t.Fatalf("unexpected request path: %s", requestPath)
	}
	if got := fields["create_list"]; got != "false" {
		t.Fatalf("expected create_list=false, got %q", got)
	}
}

func TestDS2AddURIReturnsTaskIDs(t *testing.T) {
	var destination string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("create_list") != "false" {
			t.Fatalf("expected create_list=false, got %q", r.URL.Query().Get("create_list"))
		}
		destination = r.URL.Query().Get("destination")
		_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":["dbid_1"]}}`))
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	taskIDs, err := c.AddURI(context.Background(), "magnet:?xt=urn:btih:abc", "downloads")
	if err != nil {
		t.Fatalf("AddURI error: %v", err)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "dbid_1" {
		t.Fatalf("unexpected task ids: %#v", taskIDs)
	}
	if destination != "downloads" {
		t.Fatalf("unexpected destination: %q", destination)
	}
}

func TestDS2AddURIEscapesJSONInput(t *testing.T) {
	uri := `https://example.com/file"name\segment?x=1&y=2`
	var decoded []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.Unmarshal([]byte(r.URL.Query().Get("url")), &decoded); err != nil {
			t.Fatalf("url parameter is not valid JSON array: %v", err)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":["dbid_1"]}}`))
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	if _, err := c.AddURI(context.Background(), uri, "downloads"); err != nil {
		t.Fatalf("AddURI error: %v", err)
	}
	if len(decoded) != 1 || decoded[0] != uri {
		t.Fatalf("unexpected decoded url payload: %#v", decoded)
	}
}

func TestDS2AddURIUsesDefaultDestinationWhenMissing(t *testing.T) {
	var destination string
	var decoded []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/webapi/DownloadStation/info.cgi":
			_, _ = w.Write([]byte(`{"success":true,"data":{"default_destination":"from_config"}}`))
		case "/webapi/entry.cgi":
			destination = r.URL.Query().Get("destination")
			if err := json.Unmarshal([]byte(r.URL.Query().Get("url")), &decoded); err != nil {
				t.Fatalf("url parameter is not valid JSON array: %v", err)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":["dbid_1"]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	taskIDs, err := c.AddURI(context.Background(), "magnet:?xt=urn:btih:abc", "")
	if err != nil {
		t.Fatalf("AddURI error: %v", err)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "dbid_1" {
		t.Fatalf("unexpected task ids: %#v", taskIDs)
	}
	if destination != "from_config" {
		t.Fatalf("unexpected destination: %q", destination)
	}
	if len(decoded) != 1 || decoded[0] != "magnet:?xt=urn:btih:abc" {
		t.Fatalf("unexpected decoded url payload: %#v", decoded)
	}
}

func TestDS2AddURIFailsWhenDefaultDestinationEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webapi/DownloadStation/info.cgi" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"default_destination":""}}`))
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	_, err := c.AddURI(context.Background(), "https://example.com/file.iso", "")
	if err == nil {
		t.Fatalf("expected error for empty default destination")
	}
	if !strings.Contains(err.Error(), "default_destination is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDS2ListUsesTaskAPIFirst(t *testing.T) {
	var apis []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apis = append(apis, r.URL.Query().Get("api"))
		resp := map[string]any{
			"success": true,
			"data": map[string]any{
				"tasks": []map[string]any{
					{"id": "dbid_1", "title": "t", "status": "paused"},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	tasks, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "dbid_1" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	if len(apis) != 1 || apis[0] != "SYNO.DownloadStation2.Task" {
		t.Fatalf("expected single Task API request, got %#v", apis)
	}
}

func TestDS2ListEmptyDoesNotFallbackToTaskList(t *testing.T) {
	var apis []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apis = append(apis, r.URL.Query().Get("api"))
		resp := map[string]any{
			"success": true,
			"data": map[string]any{
				"offset": 0,
				"total":  0,
				"task":   []map[string]any{},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	tasks, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected empty task list, got %#v", tasks)
	}
	if len(apis) != 1 || apis[0] != "SYNO.DownloadStation2.Task" {
		t.Fatalf("expected single Task API request, got %#v", apis)
	}
}

func TestDS2ListSupportsNumericStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"success": true,
			"data": map[string]any{
				"task": []map[string]any{
					{"id": "dbid_1", "title": "t", "status": 3},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	tasks, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Status != "3" {
		t.Fatalf("unexpected status: %q", tasks[0].Status)
	}
}

func TestDS2GetUsesGetMethodAndJSONArrayID(t *testing.T) {
	var method, idParam, additional string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		method = q.Get("method")
		idParam = q.Get("id")
		additional = q.Get("additional")
		resp := map[string]any{
			"success": true,
			"data": map[string]any{
				"task": []map[string]any{
					{"id": "dbid_3887", "title": "t", "status": 3},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()
	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	task, err := c.Get(context.Background(), "dbid_3887")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if task.ID != "dbid_3887" {
		t.Fatalf("unexpected task id: %q", task.ID)
	}
	if method != "get" {
		t.Fatalf("expected method=get, got %q", method)
	}
	if idParam != "[\"dbid_3887\"]" {
		t.Fatalf("expected JSON-array id, got %q", idParam)
	}
	if additional == "" || additional[0] != '[' {
		t.Fatalf("expected JSON-array additional, got %q", additional)
	}
}

func TestDS2AddTorrentEscapesDestinationJSON(t *testing.T) {
	destination := `my "downloads"\folder`
	var destinationRaw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader error: %v", err)
		}
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			if part.FileName() != "" {
				continue
			}
			b, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read part error: %v", err)
			}
			if part.FormName() == "destination" {
				destinationRaw = string(b)
			}
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":["dbid_1"]}}`))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	torrentPath := filepath.Join(tmpDir, "x.torrent")
	if err := os.WriteFile(torrentPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write torrent file: %v", err)
	}

	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	taskIDs, err := c.AddTorrent(context.Background(), torrentPath, destination)
	if err != nil {
		t.Fatalf("AddTorrent error: %v", err)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "dbid_1" {
		t.Fatalf("unexpected task ids: %#v", taskIDs)
	}

	var decoded string
	if err := json.Unmarshal([]byte(destinationRaw), &decoded); err != nil {
		t.Fatalf("destination is not valid JSON string: %v", err)
	}
	if decoded != destination {
		t.Fatalf("unexpected decoded destination: %q", decoded)
	}
}

func TestDS2AddTorrentUsesDefaultDestinationWhenMissing(t *testing.T) {
	var destinationRaw string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/webapi/DownloadStation/info.cgi":
			_, _ = w.Write([]byte(`{"success":true,"data":{"default_destination":"from_config"}}`))
		case "/webapi/entry.cgi/SYNO.DownloadStation2.Task":
			mr, err := r.MultipartReader()
			if err != nil {
				t.Fatalf("MultipartReader error: %v", err)
			}
			for {
				part, err := mr.NextPart()
				if err != nil {
					break
				}
				if part.FileName() != "" {
					continue
				}
				b, err := io.ReadAll(part)
				if err != nil {
					t.Fatalf("read part error: %v", err)
				}
				if part.FormName() == "destination" {
					destinationRaw = string(b)
				}
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":["dbid_1"]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	torrentPath := filepath.Join(tmpDir, "x.torrent")
	if err := os.WriteFile(torrentPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write torrent file: %v", err)
	}

	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	taskIDs, err := c.AddTorrent(context.Background(), torrentPath, "")
	if err != nil {
		t.Fatalf("AddTorrent error: %v", err)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "dbid_1" {
		t.Fatalf("unexpected task ids: %#v", taskIDs)
	}

	var decoded string
	if err := json.Unmarshal([]byte(destinationRaw), &decoded); err != nil {
		t.Fatalf("destination is not valid JSON string: %v", err)
	}
	if decoded != "from_config" {
		t.Fatalf("unexpected decoded destination: %q", decoded)
	}
}

func TestDS2AddTorrentFailsWhenDefaultDestinationEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webapi/DownloadStation/info.cgi" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"default_destination":""}}`))
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	torrentPath := filepath.Join(tmpDir, "x.torrent")
	if err := os.WriteFile(torrentPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write torrent file: %v", err)
	}

	c := newDS2TestClient(t, ts.URL, "sidv", ts.Client())
	_, err := c.AddTorrent(context.Background(), torrentPath, "")
	if err == nil {
		t.Fatalf("expected error for empty default destination")
	}
	if !strings.Contains(err.Error(), "default_destination is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
