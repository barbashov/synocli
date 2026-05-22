package system

import (
	"context"
	"net/http"
	"testing"
)

const fixtureUtilizationBody = `{"data":{"cpu":{"15min_load":92,"1min_load":106,"5min_load":93,"device":"System","other_load":4,"system_load":1,"user_load":4},"disk":{"disk":[{"device":"sda","display_name":"Drive 1","read_access":13,"read_byte":683349,"type":"internal","utilization":8,"write_access":0,"write_byte":5461},{"device":"sdb","display_name":"Drive 2","read_access":5,"read_byte":105813,"type":"internal","utilization":4,"write_access":0,"write_byte":5461}],"total":{"device":"total","read_access":22,"read_byte":879956,"utilization":3,"write_access":0,"write_byte":27305}},"lun":[],"memory":{"avail_real":273872,"avail_swap":1832660,"buffer":58280,"cached":282376,"device":"Memory","memory_size":1048576,"real_usage":39,"si_disk":0,"so_disk":0,"swap_usage":12,"total_real":1014672,"total_swap":2097084},"network":[{"device":"total","rx":1264,"tx":1311},{"device":"eth0","rx":1264,"tx":1311},{"device":"eth1","rx":0,"tx":0}],"space":{"total":{"device":"total","read_access":0,"read_byte":0,"utilization":0,"write_access":0,"write_byte":0},"volume":[]},"time":1779397015},"success":true}`

func TestGetUtilizationDecodesFixture(t *testing.T) {
	var query, cookie string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if ck, err := r.Cookie("id"); err == nil {
			cookie = ck.Value
		}
		_, _ = w.Write([]byte(fixtureUtilizationBody))
	})

	u, err := c.GetUtilization(context.Background())
	if err != nil {
		t.Fatalf("GetUtilization: %v", err)
	}
	assertCommonRequest(t, query, cookie, "SYNO.Core.System.Utilization", "1", "get")

	if u.CPU.UserLoad != 4 || u.CPU.SystemLoad != 1 || u.CPU.OtherLoad != 4 {
		t.Errorf("CPU loads = %+v", u.CPU)
	}
	if u.CPU.Load1Min != 106 || u.CPU.Load5Min != 93 || u.CPU.Load15Min != 92 {
		t.Errorf("CPU load averages = %+v", u.CPU)
	}
	if u.Memory.RealUsage != 39 || u.Memory.SwapUsage != 12 {
		t.Errorf("Memory usage = %+v", u.Memory)
	}
	if u.Memory.TotalReal != 1014672 || u.Memory.AvailReal != 273872 {
		t.Errorf("Memory totals = %+v", u.Memory)
	}
	if len(u.Disk.Disks) != 2 {
		t.Fatalf("Disks len = %d, want 2", len(u.Disk.Disks))
	}
	if u.Disk.Disks[0].Device != "sda" || u.Disk.Disks[0].DisplayName != "Drive 1" || u.Disk.Disks[0].Utilization != 8 {
		t.Errorf("Disk[0] = %+v", u.Disk.Disks[0])
	}
	if u.Disk.Total.Utilization != 3 {
		t.Errorf("Disk total utilization = %d", u.Disk.Total.Utilization)
	}
	if len(u.Network) != 3 || u.Network[0].Device != "total" || u.Network[1].Device != "eth0" {
		t.Errorf("Network = %+v", u.Network)
	}
}
