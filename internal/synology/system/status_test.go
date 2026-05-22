package system

import (
	"context"
	"net/http"
	"testing"
)

func TestGetNeedRebootDecodesFixture(t *testing.T) {
	var query, cookie string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			cookie = ck.Value
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"need_reboot":true}}`))
	})
	nr, err := c.GetNeedReboot(context.Background())
	if err != nil {
		t.Fatalf("GetNeedReboot: %v", err)
	}
	assertCommonRequest(t, query, cookie, "SYNO.Core.Hardware.NeedReboot", "1", "get")
	if !nr.NeedReboot {
		t.Errorf("NeedReboot = false, want true")
	}
}

func TestGetSystemStatusDecodesFixture(t *testing.T) {
	var query, cookie string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			cookie = ck.Value
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"is_system_crashed":false,"upgrade_ready":true}}`))
	})
	st, err := c.GetSystemStatus(context.Background())
	if err != nil {
		t.Fatalf("GetSystemStatus: %v", err)
	}
	assertCommonRequest(t, query, cookie, "SYNO.Core.System.Status", "1", "get")
	if st.IsSystemCrashed || !st.UpgradeReady {
		t.Errorf("status = %+v", st)
	}
}
