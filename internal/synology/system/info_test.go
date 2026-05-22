package system

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Captured verbatim from the live DS1511+ NAS probe.
const fixtureDSMInfoBody = `{"data":{"codepage":"enu","model":"DS1511+","ram":1024,"serial":"BAJ4N02471","temperature":54,"temperature_warn":false,"time":"Thu May 21 23:56:50 2026","uptime":1676297,"version":"25556","version_string":"DSM 6.2.4-25556 Update 7"},"success":true}`

const fixtureSystemInfoBody = `{"data":{"cpu_clock_speed":1794,"cpu_cores":"2","cpu_family":"Atom","cpu_series":"D525","cpu_vendor":"INTEL","enabled_ntp":true,"firmware_date":"2023/04/21","firmware_ver":"DSM 6.2.4-25556 Update 7","model":"DS1511+","ntp_server":"pool.ntp.org","ram_size":1024,"sata_dev":[],"serial":"BAJ4N02471","support_esata":"yes","sys_temp":55,"sys_tempwarn":false,"systempwarn":false,"temperature_warning":false,"time":"2026-05-21 23:56:52","time_zone":"Moscow","time_zone_desc":"(GMT+03:00) Moscow, St. Petersburg, Kazan","up_time":"465:38:18","usb_dev":[]},"success":true}`

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "test-sid", srv.Client(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

func assertCommonRequest(t *testing.T, query, cookie, wantAPI, wantVersion, wantMethod string) {
	t.Helper()
	for _, want := range []string{
		"api=" + wantAPI,
		"version=" + wantVersion,
		"method=" + wantMethod,
		"_sid=test-sid",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("query missing %q: %s", want, query)
		}
	}
	if cookie != "test-sid" {
		t.Errorf("cookie id = %q, want test-sid", cookie)
	}
}

func TestGetDSMInfoDecodesFixture(t *testing.T) {
	var query, cookie string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			cookie = ck.Value
		}
		_, _ = w.Write([]byte(fixtureDSMInfoBody))
	})

	info, err := c.GetDSMInfo(context.Background())
	if err != nil {
		t.Fatalf("GetDSMInfo: %v", err)
	}
	assertCommonRequest(t, query, cookie, "SYNO.DSM.Info", "2", "getinfo")
	if info.Model != "DS1511+" {
		t.Errorf("Model = %q", info.Model)
	}
	if info.Serial != "BAJ4N02471" {
		t.Errorf("Serial = %q", info.Serial)
	}
	if info.VersionString != "DSM 6.2.4-25556 Update 7" {
		t.Errorf("VersionString = %q", info.VersionString)
	}
	if info.Uptime != 1676297 {
		t.Errorf("Uptime = %d", info.Uptime)
	}
	if info.Temperature != 54 || info.TemperatureWarn {
		t.Errorf("Temperature = %d warn=%v", info.Temperature, info.TemperatureWarn)
	}
	if info.RAM != 1024 {
		t.Errorf("RAM = %d", info.RAM)
	}
}

func TestGetSystemInfoDecodesFixture(t *testing.T) {
	var query, cookie string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			cookie = ck.Value
		}
		_, _ = w.Write([]byte(fixtureSystemInfoBody))
	})

	info, err := c.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	assertCommonRequest(t, query, cookie, "SYNO.Core.System", "3", "info")
	if info.CPUFamily != "Atom" || info.CPUSeries != "D525" || info.CPUVendor != "INTEL" {
		t.Errorf("CPU = %q %q %q", info.CPUVendor, info.CPUFamily, info.CPUSeries)
	}
	if info.CPUClockSpeed != 1794 {
		t.Errorf("CPUClockSpeed = %d", info.CPUClockSpeed)
	}
	if int(info.CPUCores) != 2 {
		t.Errorf("CPUCores = %d (DSM string form)", info.CPUCores)
	}
	if info.RAMSize != 1024 {
		t.Errorf("RAMSize = %d", info.RAMSize)
	}
	if info.TimeZone != "Moscow" {
		t.Errorf("TimeZone = %q", info.TimeZone)
	}
	if !info.EnabledNTP || info.NTPServer != "pool.ntp.org" {
		t.Errorf("NTP enabled=%v server=%q", info.EnabledNTP, info.NTPServer)
	}
}

func TestGetSystemInfoAcceptsNumericCPUCores(t *testing.T) {
	// Some DSM versions return cpu_cores as a JSON number, not a string.
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"cpu_cores":4,"model":"DS920+"}}`))
	})
	info, err := c.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	if int(info.CPUCores) != 4 {
		t.Errorf("CPUCores = %d, want 4", info.CPUCores)
	}
}

func TestGetDSMInfoPropagatesAPIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error":{"code":105}}`))
	})
	_, err := c.GetDSMInfo(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 105 {
		t.Errorf("code = %d, want 105", apiErr.Code)
	}
	if apiErr.API != "SYNO.DSM.Info" {
		t.Errorf("API = %q", apiErr.API)
	}
}
