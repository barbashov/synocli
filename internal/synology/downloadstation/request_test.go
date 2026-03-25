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

func TestPauseBuildsExpectedQuery(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer ts.Close()
	c := &Client{Endpoint: ts.URL, Path: "/task", Version: 1, HTTP: ts.Client()}
	if err := c.Pause(context.Background(), "sid123", []string{"a", "b"}); err != nil {
		t.Fatalf("Pause error: %v", err)
	}
	if !strings.Contains(rawQuery, "method=pause") || !strings.Contains(rawQuery, "id=a%2Cb") || !strings.Contains(rawQuery, "_sid=sid123") {
		t.Fatalf("unexpected query: %s", rawQuery)
	}
}

func TestListIncludesUnlimitedPaging(t *testing.T) {
	var rawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"success":true,"data":{"tasks":[]}}`))
	}))
	defer ts.Close()
	c := &Client{Endpoint: ts.URL, Path: "/task", Version: 1, HTTP: ts.Client()}
	if _, err := c.List(context.Background(), "sid123"); err != nil {
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
	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	_, err := c.AddTorrent(context.Background(), "sidv", torrentPath, "downloads")
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("create_list") != "false" {
			t.Fatalf("expected create_list=false, got %q", r.URL.Query().Get("create_list"))
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"task_id":["dbid_1"]}}`))
	}))
	defer ts.Close()
	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	taskIDs, err := c.AddURI(context.Background(), "sidv", "magnet:?xt=urn:btih:abc", "downloads")
	if err != nil {
		t.Fatalf("AddURI error: %v", err)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "dbid_1" {
		t.Fatalf("unexpected task ids: %#v", taskIDs)
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
	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	if _, err := c.AddURI(context.Background(), "sidv", uri, "downloads"); err != nil {
		t.Fatalf("AddURI error: %v", err)
	}
	if len(decoded) != 1 || decoded[0] != uri {
		t.Fatalf("unexpected decoded url payload: %#v", decoded)
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
	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	tasks, err := c.List(context.Background(), "sidv")
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
	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	tasks, err := c.List(context.Background(), "sidv")
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
	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	tasks, err := c.List(context.Background(), "sidv")
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
	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	task, err := c.Get(context.Background(), "sidv", "dbid_3887")
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

	c := &Client{
		Endpoint: ts.URL,
		Path:     "/webapi/entry.cgi",
		Version:  2,
		APIName:  "SYNO.DownloadStation2.Task",
		HTTP:     ts.Client(),
	}
	taskIDs, err := c.AddTorrent(context.Background(), "sidv", torrentPath, destination)
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
