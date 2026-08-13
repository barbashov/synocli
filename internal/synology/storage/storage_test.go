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
      {"id":"reuse_1","status":"repairing","raidType":"single","device_type":"raid_5","disks":["sda","sdb","sdc","sdd","sde"],
       "is_actioning":true,"progress":{"percent":"7.10","step":"raid_parity_checking"},
       "raids":[
         {"raidPath":"/dev/md2","hasParity":true,"normalDevCount":4,"designedDiskCount":5,
          "devices":[
            {"id":"sde","slot":4,"status":"normal"},
            {"id":"sdb","slot":1,"status":"rebuild"},
            {"id":"sda","slot":0,"status":"normal"}
          ]}
       ],
       "size":{"total":"11983029272576","used":"11983029272576"}}
    ],
    "volumes": [
      {"id":"volume_1","vol_path":"/volume1","status":"repairing","fs_type":"ext4","raidType":"single","device_type":"raid_5",
       "pool_path":"reuse_1","dev_path":"/dev/md2","is_actioning":true,
       "progress":{"percent":"7.10","step":"raid_parity_checking"},
       "size":{"total":"11794790506496","used":"6529575075840"}}
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
	v0 := info.Volumes[0]
	if v0.DeviceType != "raid_5" || v0.PoolPath != "reuse_1" || v0.DevPath != "/dev/md2" || !v0.IsActioning {
		t.Errorf("Volume[0] extras = %+v", v0)
	}
	if v0.Progress.Percent != "7.10" || v0.Progress.Step != "raid_parity_checking" {
		t.Errorf("Volume[0] progress = %+v", v0.Progress)
	}
	if len(info.StoragePools) != 1 || len(info.StoragePools[0].Disks) != 5 {
		t.Fatalf("Pools = %+v", info.StoragePools)
	}
	p0 := info.StoragePools[0]
	if p0.Status != "repairing" || p0.DeviceType != "raid_5" || !p0.IsActioning {
		t.Errorf("Pool[0] = %+v", p0)
	}
	if p0.Progress.Percent != "7.10" || p0.Progress.Step != "raid_parity_checking" {
		t.Errorf("Pool[0] progress = %+v", p0.Progress)
	}
	if len(p0.Raids) != 1 {
		t.Fatalf("Pool[0] raids = %+v", p0.Raids)
	}
	r0 := p0.Raids[0]
	if r0.RaidPath != "/dev/md2" || !r0.HasParity || r0.NormalDevCount != 4 || r0.DesignedDiskCount != 5 {
		t.Errorf("Raid[0] = %+v", r0)
	}
	if len(r0.Devices) != 3 || r0.Devices[1].ID != "sdb" || r0.Devices[1].Slot != 1 || r0.Devices[1].Status != "rebuild" {
		t.Errorf("Raid[0] devices = %+v", r0.Devices)
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

// Regression: the Smart API must send its own version, not the (separately
// discovered/clamped) Storage API version held in c.version.
func TestGetHealthInfoUsesSmartVersionNotStorageVersion(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(fixtureHealthInfoBody))
	}))
	t.Cleanup(srv.Close)
	// Storage API discovered as version 5; Smart must still be requested at v1.
	c, err := NewClient(srv.URL, "test-sid", srv.Client(), "", 5)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.GetHealthInfo(context.Background(), "/dev/sda", "sda"); err != nil {
		t.Fatalf("GetHealthInfo: %v", err)
	}
	if !strings.Contains(query, "version=1") || strings.Contains(query, "version=5") {
		t.Fatalf("Smart call used wrong version: %s", query)
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

func TestLoadInfoHealthyPoolWithoutProgress(t *testing.T) {
	// Healthy pools/volumes omit progress and raids entirely; decode must
	// yield zero values, not errors.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"storagePools":[{"id":"reuse_1","status":"normal","device_type":"raid_5"}],"volumes":[{"id":"v","vol_path":"/volume1","status":"normal"}]}}`))
	})
	info, err := c.LoadInfo(context.Background())
	if err != nil {
		t.Fatalf("LoadInfo: %v", err)
	}
	p := info.StoragePools[0]
	if p.IsActioning || p.Progress.Percent != "" || p.Progress.Step != "" || len(p.Raids) != 0 {
		t.Errorf("pool = %+v", p)
	}
	if info.Volumes[0].Progress.Percent != "" || info.Volumes[0].IsActioning {
		t.Errorf("volume = %+v", info.Volumes[0])
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
