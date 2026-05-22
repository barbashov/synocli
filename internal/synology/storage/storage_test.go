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
// The second disk row is synthesised as an SSD to exercise IsSSD/RemainLife.
const fixtureLoadInfoBody = `{
  "success": true,
  "data": {
    "disks": [
      {"id":"sda","name":"Drive 1","device":"/dev/sda","model":"WD30EZRZ-00Z5HB0","vendor":"WDC     ","status":"normal","smart_status":"normal","size_total":"3000592982016","temp":42,
       "diskType":"SATA","isSsd":false,"slot_id":1,"serial":"WD-WCC4N7EAT5S5","firm":"80.00A80","used_by":"reuse_1",
       "exceed_bad_sector_thr":false,"below_remain_life_thr":false,"remain_life":-1,
       "container":{"order":0,"str":"DS1511+","type":"internal"}},
      {"id":"sdb","name":"Drive 2","device":"/dev/sdb","model":"SSD-EXAMPLE","vendor":"TOSHIBA ","status":"normal","smart_status":"normal","size_total":"500107862016","temp":35,
       "diskType":"SATA","isSsd":true,"slot_id":2,"serial":"SSD-SERIAL","firm":"1.0","used_by":"",
       "exceed_bad_sector_thr":false,"below_remain_life_thr":false,"remain_life":88,
       "container":{"order":0,"str":"DS1511+","type":"internal"}}
    ],
    "storagePools": [
      {"id":"reuse_1","status":"normal","raidType":"single","disks":["sda","sdb","sdc","sdd","sde"],"size":{"total":"11983029272576","used":"11983029272576"}}
    ],
    "volumes": [
      {"id":"volume_1","vol_path":"/volume1","status":"normal","fs_type":"ext4","raidType":"single","size":{"total":"11794790506496","used":"6529575075840"}}
    ]
  }
}`

// Trimmed from a live get_health_info response (DSM 6.2.4, WD30EZRZ).
const fixtureHealthInfoBody = `{
  "success": true,
  "data": {
    "healthInfo": {
      "count": 17,
      "overview": {
        "smart": "normal",
        "smart_test": "normal",
        "overview_status": "normal",
        "poweron": "48028",
        "idnf": 0,
        "retry": 0,
        "unc": 0,
        "remain_life": -1,
        "exceed_bad_sector_thr": false,
        "below_remain_life_thr": false,
        "isSsd": false,
        "isNVMeDisk": false,
        "read_only": false,
        "smart_schedule_list": [
          {"next_trigger_time": "2026-06-01 05:00"},
          {"next_trigger_time": "2026-05-26 05:00"}
        ]
      },
      "smartInfo": [
        {"id":"1","name":"Raw_Read_Error_Rate","current":"200","worst":"200","threshold":"051","raw":"0","status":"OK"},
        {"id":"5","name":"Reallocated_Sector_Ct","current":"200","worst":"200","threshold":"140","raw":"0","status":"OK"},
        {"id":"9","name":"Power_On_Hours","current":"34","worst":"34","threshold":"000","raw":"48028","status":"OK"},
        {"id":"197","name":"Current_Pending_Sector","current":"200","worst":"200","threshold":"000","raw":"0","status":"OK"}
      ]
    }
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
	if len(info.Disks) != 2 || info.Disks[0].ID != "sda" || info.Disks[1].Temp != 35 {
		t.Fatalf("Disks = %+v", info.Disks)
	}
	if info.Disks[0].SizeTotal.Int64() != 3000592982016 {
		t.Errorf("Disk total = %d", info.Disks[0].SizeTotal.Int64())
	}
	d0 := info.Disks[0]
	if d0.Device != "/dev/sda" || d0.DiskType != "SATA" || d0.IsSSD || d0.SlotID != 1 ||
		d0.Serial != "WD-WCC4N7EAT5S5" || d0.Firm != "80.00A80" || d0.UsedBy != "reuse_1" ||
		d0.RemainLife != -1 || d0.Container.Str != "DS1511+" || d0.Container.Type != "internal" {
		t.Errorf("Disk[0] extras = %+v", d0)
	}
	d1 := info.Disks[1]
	if !d1.IsSSD || d1.RemainLife != 88 || d1.UsedBy != "" {
		t.Errorf("Disk[1] (ssd) = %+v", d1)
	}
}

func TestGetHealthInfoDecodesFixture(t *testing.T) {
	var query, cookie string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			cookie = ck.Value
		}
		_, _ = w.Write([]byte(fixtureHealthInfoBody))
	})

	hi, err := c.GetHealthInfo(context.Background(), "/dev/sda", "sda")
	if err != nil {
		t.Fatalf("GetHealthInfo: %v", err)
	}

	for _, want := range []string{
		"api=SYNO.Storage.CGI.Smart",
		"version=1",
		"method=get_health_info",
		"_sid=test-sid",
		"device=%2Fdev%2Fsda",
		"disk=sda",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query missing %q: %s", want, query)
		}
	}
	if cookie != "test-sid" {
		t.Errorf("cookie id = %q", cookie)
	}
	if hi.Overview.Smart != "normal" || hi.Overview.Poweron.Int64() != 48028 {
		t.Errorf("overview = %+v", hi.Overview)
	}
	if len(hi.Overview.SmartScheduleList) != 2 ||
		hi.Overview.SmartScheduleList[0].NextTriggerTime != "2026-06-01 05:00" {
		t.Errorf("schedule = %+v", hi.Overview.SmartScheduleList)
	}
	if len(hi.SmartInfo) != 4 {
		t.Fatalf("smartInfo len = %d", len(hi.SmartInfo))
	}
	attr := hi.SmartInfo[1]
	if attr.ID != "5" || attr.Name != "Reallocated_Sector_Ct" || attr.Raw != "0" || attr.Status != "OK" {
		t.Errorf("attr[1] = %+v", attr)
	}
}

func TestGetHealthInfoRequiresDeviceAndDiskID(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("should not call DSM, got %s", r.URL.RawQuery)
	})
	if _, err := c.GetHealthInfo(context.Background(), "", "sda"); err == nil {
		t.Errorf("expected error for empty device")
	}
	if _, err := c.GetHealthInfo(context.Background(), "/dev/sda", ""); err == nil {
		t.Errorf("expected error for empty diskID")
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
