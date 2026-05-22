package storage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Captured from the live DS1511+ NAS, trimmed to the fields the client models.
const fixtureLoadInfoBody = `{
  "success": true,
  "data": {
    "disks": [
      {"id":"sda","name":"Drive 1","model":"WD30EZRZ-00Z5HB0","vendor":"WDC     ","status":"normal","smart_status":"normal","size_total":"3000592982016","temp":42},
      {"id":"sdb","name":"Drive 2","model":"HDWU130","vendor":"TOSHIBA ","status":"normal","smart_status":"normal","size_total":"3000592982016","temp":43}
    ],
    "storagePools": [
      {"id":"reuse_1","status":"normal","raidType":"single","disks":["sda","sdb","sdc","sdd","sde"],"size":{"total":"11983029272576","used":"11983029272576"}}
    ],
    "volumes": [
      {"id":"volume_1","vol_path":"/volume1","status":"normal","fs_type":"ext4","raidType":"single","size":{"total":"11794790506496","used":"6529575075840"}}
    ]
  }
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "test-sid", srv.Client(), "", 0)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestLoadInfoDecodesFixture(t *testing.T) {
	var query, cookie string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			cookie = ck.Value
		}
		_, _ = w.Write([]byte(fixtureLoadInfoBody))
	})

	info, err := c.LoadInfo(context.Background())
	if err != nil {
		t.Fatalf("LoadInfo: %v", err)
	}

	for _, want := range []string{"api=SYNO.Storage.CGI.Storage", "version=1", "method=load_info", "_sid=test-sid"} {
		if !strings.Contains(query, want) {
			t.Errorf("query missing %q: %s", want, query)
		}
	}
	if cookie != "test-sid" {
		t.Errorf("cookie id = %q", cookie)
	}
	if len(info.Volumes) != 1 || info.Volumes[0].VolPath != "/volume1" {
		t.Fatalf("Volumes = %+v", info.Volumes)
	}
	if info.Volumes[0].Size.Total.Int64() != 11794790506496 {
		t.Errorf("Volume total = %d", info.Volumes[0].Size.Total.Int64())
	}
	if info.Volumes[0].Size.Used.Int64() != 6529575075840 {
		t.Errorf("Volume used = %d", info.Volumes[0].Size.Used.Int64())
	}
	if len(info.StoragePools) != 1 || len(info.StoragePools[0].Disks) != 5 {
		t.Fatalf("Pools = %+v", info.StoragePools)
	}
	if len(info.Disks) != 2 || info.Disks[0].ID != "sda" || info.Disks[1].Temp != 43 {
		t.Fatalf("Disks = %+v", info.Disks)
	}
	if info.Disks[0].SizeTotal.Int64() != 3000592982016 {
		t.Errorf("Disk total = %d", info.Disks[0].SizeTotal.Int64())
	}
}

func TestLoadInfoAcceptsNumericSizes(t *testing.T) {
	// Some DSM versions return size totals as JSON numbers instead of strings.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"volumes":[{"id":"v","vol_path":"/volume1","size":{"total":1000,"used":250}}]}}`))
	})
	info, err := c.LoadInfo(context.Background())
	if err != nil {
		t.Fatalf("LoadInfo: %v", err)
	}
	if info.Volumes[0].Size.Total.Int64() != 1000 || info.Volumes[0].Size.Used.Int64() != 250 {
		t.Errorf("sizes = %+v", info.Volumes[0].Size)
	}
}

func TestLoadInfoPropagatesAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":105}}`))
	})
	_, err := c.LoadInfo(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 105 {
		t.Errorf("code = %d, want 105", apiErr.Code)
	}
}
