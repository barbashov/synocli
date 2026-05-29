package downloadstation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Captured verbatim from a live DSM 7 NAS via the Step-0 probe: a 3-file BT
// task (Big Buck Bunny) with index 0 set to priority "high" and index 2 set to
// wanted=false, so the fixture exercises every field and both wanted states.
const fixtureBTFileListBody = `{"data":{"items":[{"index":0,"name":"Big Buck Bunny.en.srt","priority":"high","size":140,"size_downloaded":140,"wanted":true},{"index":1,"name":"Big Buck Bunny.mp4","priority":"normal","size":276134947,"size_downloaded":276134947,"wanted":true},{"index":2,"name":"poster.jpg","priority":"normal","size":310380,"size_downloaded":0,"wanted":false}],"limit":-1,"offset":0,"total":3},"success":true}`

func newBTFileTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "test-sid", srv.Client(), "", 0, "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestListBTFilesDecodesFixture(t *testing.T) {
	var capturedPath, capturedQuery, capturedCookie string
	c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			capturedCookie = ck.Value
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureBTFileListBody))
	})

	files, err := c.ListBTFiles(context.Background(), "dbid_4065")
	if err != nil {
		t.Fatalf("ListBTFiles: %v", err)
	}

	if capturedPath != btFilePath {
		t.Errorf("path = %q, want %q", capturedPath, btFilePath)
	}
	for _, want := range []string{
		"api=SYNO.DownloadStation2.Task.BT.File",
		"version=2",
		"method=list",
		"task_id=dbid_4065",
		"limit=-1",
		"_sid=test-sid",
	} {
		if !strings.Contains(capturedQuery, want) {
			t.Errorf("query missing %q: %s", want, capturedQuery)
		}
	}
	if capturedCookie != "test-sid" {
		t.Errorf("cookie id = %q, want test-sid", capturedCookie)
	}

	if len(files) != 3 {
		t.Fatalf("len(files) = %d, want 3", len(files))
	}
	if files[0].Index != 0 || files[0].Name != "Big Buck Bunny.en.srt" || files[0].Priority != "high" || !files[0].Wanted {
		t.Errorf("file[0] = %+v", files[0])
	}
	if files[1].Size != 276134947 || files[1].SizeDownloaded != 276134947 {
		t.Errorf("file[1] sizes = %d/%d", files[1].Size, files[1].SizeDownloaded)
	}
	if files[2].Index != 2 || files[2].Name != "poster.jpg" || files[2].Wanted || files[2].SizeDownloaded != 0 {
		t.Errorf("file[2] = %+v (want wanted=false, downloaded=0)", files[2])
	}
}

func TestListBTFilesEmpty(t *testing.T) {
	// A magnet whose metadata has not arrived (or a non-multi-file task)
	// returns success with an empty item list, not an error.
	c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"items":[],"limit":-1,"offset":0,"total":0},"success":true}`))
	})
	files, err := c.ListBTFiles(context.Background(), "dbid_1")
	if err != nil {
		t.Fatalf("ListBTFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("len(files) = %d, want 0", len(files))
	}
}

func TestListBTFilesPropagatesAPIError(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		code    int
		wantMsg string
	}{
		// Unknown task_id (probe-confirmed).
		{name: "unknown task", body: `{"error":{"code":404},"success":false}`, code: 404, wantMsg: "invalid task id"},
		// A just-added torrent whose metadata has not been parsed yet returns
		// 1913 until the file list is ready (probe-confirmed).
		{name: "metadata not ready", body: `{"error":{"code":1913},"success":false}`, code: 1913, wantMsg: "BT file list not ready"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := c.ListBTFiles(context.Background(), "dbid_x")
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}
			if apiErr.Code != tc.code {
				t.Errorf("code = %d, want %d", apiErr.Code, tc.code)
			}
			if !strings.Contains(apiErr.Error(), tc.wantMsg) {
				t.Errorf("message %q missing %q", apiErr.Error(), tc.wantMsg)
			}
		})
	}
}

func TestSetBTFileWantedEncodesIndexArray(t *testing.T) {
	tests := []struct {
		name    string
		indices []int64
		wanted  bool
		wantIdx string
		wantWnt string
	}{
		{name: "skip multiple", indices: []int64{0, 2, 5}, wanted: false, wantIdx: "index=[0,2,5]", wantWnt: "wanted=false"},
		{name: "include single", indices: []int64{1}, wanted: true, wantIdx: "index=[1]", wantWnt: "wanted=true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var capturedQuery string
			c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				// DSM expects the raw form value; decode before substring checks.
				capturedQuery, _ = url.QueryUnescape(r.URL.RawQuery)
				_, _ = w.Write([]byte(`{"success":true}`))
			})
			if err := c.SetBTFileWanted(context.Background(), "dbid_4065", tc.indices, tc.wanted); err != nil {
				t.Fatalf("SetBTFileWanted: %v", err)
			}
			for _, want := range []string{
				"api=SYNO.DownloadStation2.Task.BT.File",
				"method=set",
				"task_id=dbid_4065",
				tc.wantIdx,
				tc.wantWnt,
			} {
				if !strings.Contains(capturedQuery, want) {
					t.Errorf("query missing %q: %s", want, capturedQuery)
				}
			}
		})
	}
}

func TestSetBTFileWantedNoIndicesIsNoop(t *testing.T) {
	c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called for empty index list")
	})
	if err := c.SetBTFileWanted(context.Background(), "dbid_4065", nil, false); err != nil {
		t.Fatalf("SetBTFileWanted(nil): %v", err)
	}
}

func TestSetBTFilePriorityEncodesParams(t *testing.T) {
	var capturedQuery string
	c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery, _ = url.QueryUnescape(r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	if err := c.SetBTFilePriority(context.Background(), "dbid_4065", []int64{0, 3}, BTFilePriorityHigh); err != nil {
		t.Fatalf("SetBTFilePriority: %v", err)
	}
	for _, want := range []string{"method=set", "index=[0,3]", "priority=high"} {
		if !strings.Contains(capturedQuery, want) {
			t.Errorf("query missing %q: %s", want, capturedQuery)
		}
	}
}

func TestSetBTFileWantedPropagatesAPIError(t *testing.T) {
	// Out-of-range index returns synology code 1911 (probe-confirmed).
	c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":1911},"success":false}`))
	})
	err := c.SetBTFileWanted(context.Background(), "dbid_4065", []int64{99}, false)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 1911 {
		t.Errorf("code = %d, want 1911", apiErr.Code)
	}
	if !strings.Contains(apiErr.Error(), "invalid file index") {
		t.Errorf("error message missing 1911 mapping: %s", apiErr.Error())
	}
}

// Regression: a multi-file torrent with more indices than fit in one URL must
// be split across multiple set requests, each carrying a subset.
func TestSetBTFileWantedChunksLargeIndexList(t *testing.T) {
	total := maxBTFileIndicesPerRequest*2 + 7
	indices := make([]int64, total)
	for i := range indices {
		indices[i] = int64(i)
	}

	var requests int
	var seen int
	c := newBTFileTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		q, _ := url.QueryUnescape(r.URL.RawQuery)
		// crude count of indices in this request's array
		start := strings.Index(q, "index=[")
		if start >= 0 {
			arr := q[start+len("index=[") :]
			if end := strings.Index(arr, "]"); end >= 0 {
				arr = arr[:end]
				if arr != "" {
					seen += strings.Count(arr, ",") + 1
				}
			}
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	if err := c.SetBTFileWanted(context.Background(), "dbid_4065", indices, false); err != nil {
		t.Fatalf("SetBTFileWanted: %v", err)
	}
	wantRequests := 3 // 500 + 500 + 7
	if requests != wantRequests {
		t.Errorf("requests = %d, want %d", requests, wantRequests)
	}
	if seen != total {
		t.Errorf("total indices sent = %d, want %d", seen, total)
	}
}

func TestChunkInt64(t *testing.T) {
	if got := chunkInt64(nil, 3); got != nil {
		t.Errorf("chunkInt64(nil) = %v, want nil", got)
	}
	got := chunkInt64([]int64{1, 2, 3, 4, 5}, 2)
	if len(got) != 3 || len(got[0]) != 2 || len(got[2]) != 1 {
		t.Errorf("unexpected chunks: %v", got)
	}
}

func TestMapBTFile(t *testing.T) {
	m := MapBTFile(BTFile{Index: 2, Name: "poster.jpg", Size: 310380, SizeDownloaded: 0, Priority: "normal", Wanted: false})
	want := map[string]any{
		"index":           int64(2),
		"name":            "poster.jpg",
		"size":            int64(310380),
		"downloaded_size": int64(0),
		"priority":        "normal",
		"wanted":          false,
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("key %q = %v (%T), want %v (%T)", k, m[k], m[k], v, v)
		}
	}
	if len(m) != len(want) {
		t.Errorf("MapBTFile keys = %d, want %d: %v", len(m), len(want), m)
	}
}
