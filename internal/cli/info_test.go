package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"synocli/internal/synology/storage"
	"synocli/internal/synology/system"
)

func TestInfoCmdHasUtilizationSubcommand(t *testing.T) {
	cmd := newInfoCmd(&appContext{})
	if cmd.Name() != "info" {
		t.Fatalf("name = %q", cmd.Name())
	}
	sub, _, err := cmd.Find([]string{"utilization"})
	if err != nil || sub == nil || sub.Name() != "utilization" {
		t.Fatalf("info utilization not found: sub=%#v err=%v", sub, err)
	}
}

func TestInfoCmdRegisteredOnRoot(t *testing.T) {
	root := newRootCmd(nil, &bytes.Buffer{}, &bytes.Buffer{})
	sub, _, err := root.Find([]string{"info"})
	if err != nil || sub == nil || sub.Name() != "info" {
		t.Fatalf("root.info not registered: %v", err)
	}
}

func TestRenderInfoHumanOverview(t *testing.T) {
	dsm := &system.DSMInfo{
		Model: "DS1511+", Serial: "BAJ4N02471",
		VersionString: "DSM 6.2.4-25556 Update 7",
		Uptime:        1676297, // ~19 days
		Temperature:   54, RAM: 1024,
	}
	sys := &system.SystemInfo{
		RAMSize: 1024, CPUClockSpeed: 1794, CPUCores: 2,
		CPUFamily: "Atom", CPUSeries: "D525", CPUVendor: "INTEL",
		Time: "2026-05-21 23:56:52", TimeZoneDesc: "(GMT+03:00) Moscow",
	}
	reboot := &system.NeedReboot{NeedReboot: false}
	store := &storage.LoadInfo{
		Volumes: []storage.Volume{{
			ID: "volume_1", VolPath: "/volume1", Status: "normal",
			FSType: "ext4", RAIDType: "single",
			Size: storage.Size{Total: 11794790506496, Used: 6529575075840},
		}},
	}

	var buf bytes.Buffer
	renderInfoHuman(&buf, dsm, sys, reboot, store)
	out := buf.String()

	for _, want := range []string{
		"System", "Model:", "DS1511+", "BAJ4N02471",
		"DSM 6.2.4-25556 Update 7", "19 days",
		"Hardware", "INTEL Atom D525", "2 cores", "1794 MHz", "1.0 GB",
		"54 °C",
		"Volumes", "/volume1", "single", "ext4", "5.9 TB",
		"Health", "Reboot Required:", " no",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestRenderInfoHumanOmitsHealthAndVolumesWhenAbsent(t *testing.T) {
	dsm := &system.DSMInfo{Model: "DS220+", Serial: "X", VersionString: "DSM 7.0"}
	sys := &system.SystemInfo{}
	var buf bytes.Buffer
	renderInfoHuman(&buf, dsm, sys, nil, nil)
	out := buf.String()
	if strings.Contains(out, "Volumes") {
		t.Errorf("Volumes section should be omitted: %s", out)
	}
	if strings.Contains(out, "Health") {
		t.Errorf("Health section should be omitted: %s", out)
	}
}

func TestRenderUtilizationHumanFull(t *testing.T) {
	util := &system.Utilization{
		CPU: system.CPUUtilization{UserLoad: 4, SystemLoad: 1, OtherLoad: 4, Load1Min: 106, Load5Min: 93, Load15Min: 92},
		Memory: system.MemoryUtilization{
			RealUsage: 39, SwapUsage: 12,
			TotalReal: 1014672, AvailReal: 273872, TotalSwap: 2097084,
		},
		Disk: system.DiskUtilizationSet{Disks: []system.DiskUtilization{
			{Device: "sda", DisplayName: "Drive 1", Utilization: 8, ReadByte: 683349, WriteByte: 5461},
		}},
		Network: []system.NetworkUsage{
			{Device: "eth0", RX: 1264, TX: 1311},
		},
	}
	dsm := &system.DSMInfo{Temperature: 54}
	status := &system.SystemStatus{IsSystemCrashed: false, UpgradeReady: false}
	store := &storage.LoadInfo{Disks: []storage.Disk{
		{ID: "sda", Model: "WD30EZRZ-00Z5HB0", Temp: 42, SmartStatus: "normal"},
	}}

	var buf bytes.Buffer
	renderUtilizationHuman(&buf, util, dsm, status, store)
	out := buf.String()

	for _, want := range []string{
		"CPU", "User:", "4%", "Load Avg", "1.06", "0.93", "0.92",
		"Memory", "Real Usage:", "39%",
		"Temperature", "System:", "54 °C",
		"Disks", "Drive 1", "WD30EZRZ-00Z5HB0", "42 °C", "normal", "8%",
		"Network", "eth0",
		"Status", "System Crashed:", "Upgrade Ready:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestInfoCmdHasDisksSubcommand(t *testing.T) {
	cmd := newInfoCmd(&appContext{})
	sub, _, err := cmd.Find([]string{"disks"})
	if err != nil || sub == nil || sub.Name() != "disks" {
		t.Fatalf("info disks not found: sub=%#v err=%v", sub, err)
	}
	smart, _, err := cmd.Find([]string{"disks", "smart"})
	if err != nil || smart == nil || smart.Name() != "smart" {
		t.Fatalf("info disks smart not found: sub=%#v err=%v", smart, err)
	}
}

func TestRenderInfoDisksHumanEmpty(t *testing.T) {
	var buf bytes.Buffer
	renderInfoDisksHuman(&buf, nil)
	if !strings.Contains(buf.String(), "No disks reported.") {
		t.Errorf("expected empty-state message, got: %s", buf.String())
	}
}

func TestRenderInfoDisksHumanCovers(t *testing.T) {
	rows := []diskRow{
		// healthy HDD
		{
			Disk: storage.Disk{
				ID: "sda", Name: "Drive 1", Model: "WD30EZRZ-00Z5HB0",
				SizeTotal: 3000592982016, Temp: 43, SlotID: 1, UsedBy: "reuse_1",
				DiskType: "SATA", RemainLife: -1, SmartStatus: "normal",
			},
			Health: &storage.HealthInfo{
				Overview: storage.HealthOverview{Smart: "normal", Poweron: 48028},
				SmartInfo: []storage.SmartAttribute{
					{ID: "5", Raw: "0"},
					{ID: "197", Raw: "0"},
				},
			},
		},
		// SMART thresholds OK but raw reallocated > 0 -> early warning
		{
			Disk: storage.Disk{
				ID: "sdc", Name: "Drive 3", Model: "WD30EZRX",
				SizeTotal: 3000592982016, Temp: 47, SlotID: 3, UsedBy: "reuse_1",
				RemainLife: -1, SmartStatus: "normal",
			},
			Health: &storage.HealthInfo{
				Overview: storage.HealthOverview{Smart: "normal", Poweron: 100000},
				SmartInfo: []storage.SmartAttribute{
					{ID: "5", Raw: "3"},
					{ID: "197", Raw: "2"},
				},
			},
		},
		// SSD with low remaining life flagged by DSM
		{
			Disk: storage.Disk{
				ID: "sde", Name: "Drive 5", Model: "SSD-X",
				SizeTotal: 500107862016, Temp: 35, SlotID: 5, UsedBy: "",
				IsSSD: true, RemainLife: 18, BelowRemainLifeThr: true,
				SmartStatus: "normal",
			},
			Health: &storage.HealthInfo{
				Overview: storage.HealthOverview{Smart: "normal", Poweron: 10000, RemainLife: 18},
			},
		},
	}

	var buf bytes.Buffer
	renderInfoDisksHuman(&buf, rows)
	out := buf.String()

	for _, want := range []string{
		"Disks",
		"BAY", "NAME", "MODEL", "SIZE", "TYPE", "TEMP", "AGE", "HEALTH", "USED BY",
		"Drive 1", "WD30EZRZ-00Z5HB0", "HDD", "43 °C", "5y 6mo", "normal", "reuse_1",
		"Drive 3", "3 bad", "2 pending",
		"Drive 5", "SSD 18%", "18% life", "WARN",
		"3 disks", "2 warnings", "1 healthy",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestRenderInfoDisksHumanHealthError(t *testing.T) {
	rows := []diskRow{{
		Disk:        storage.Disk{ID: "sda", Name: "Drive 1", SlotID: 1, RemainLife: -1, SmartStatus: "normal"},
		HealthError: fmt.Errorf("api error"),
	}}
	var buf bytes.Buffer
	renderInfoDisksHuman(&buf, rows)
	out := buf.String()
	if !strings.Contains(out, "error") || !strings.Contains(out, "WARN") {
		t.Errorf("expected error + WARN in HEALTH column, got: %s", out)
	}
	if !strings.Contains(out, "1 warning") {
		t.Errorf("expected 1 warning in summary, got: %s", out)
	}
}

func TestRenderInfoDisksSmartHuman(t *testing.T) {
	rows := []diskRow{{
		Disk: storage.Disk{
			ID: "sda", Name: "Drive 1", Model: "WD30EZRZ", Serial: "WD-X", Firm: "80.00",
			SizeTotal: 3000592982016, SlotID: 1, UsedBy: "reuse_1", RemainLife: -1,
		},
		Health: &storage.HealthInfo{
			Overview: storage.HealthOverview{
				Smart: "normal", SmartTest: "normal", Poweron: 48028,
				SmartScheduleList: []storage.SmartScheduleEntry{{NextTriggerTime: "2026-06-01 05:00"}},
			},
			SmartInfo: []storage.SmartAttribute{
				{ID: "5", Name: "Reallocated_Sector_Ct", Current: "200", Worst: "200", Threshold: "140", Raw: "0", Status: "OK"},
				{ID: "9", Name: "Power_On_Hours", Current: "34", Worst: "34", Threshold: "000", Raw: "48028", Status: "OK"},
			},
		},
	}}
	var buf bytes.Buffer
	renderInfoDisksSmartHuman(&buf, rows)
	out := buf.String()
	for _, want := range []string{
		"Disk: Drive 1 (sda)", "Model:", "WD30EZRZ", "Serial:", "WD-X",
		"Firmware:", "80.00", "SMART Status:", "Power On:", "48028h (5y 6mo)",
		"Next Tests:", "2026-06-01 05:00",
		"Attributes", "Reallocated_Sector_Ct", "Power_On_Hours", "OK",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestRenderInfoDisksSmartHumanError(t *testing.T) {
	rows := []diskRow{{
		Disk:        storage.Disk{ID: "sda", Name: "Drive 1", RemainLife: -1},
		HealthError: fmt.Errorf("api error"),
	}}
	var buf bytes.Buffer
	renderInfoDisksSmartHuman(&buf, rows)
	out := buf.String()
	if !strings.Contains(out, "Disk: Drive 1 (sda)") || !strings.Contains(out, "api error") {
		t.Errorf("expected disk header + error line, got: %s", out)
	}
	if strings.Contains(out, "Attributes") {
		t.Errorf("attribute table should be omitted on error: %s", out)
	}
}

func TestFormatPowerOnAge(t *testing.T) {
	cases := map[int64]string{
		0:            "-",
		-5:           "-",
		3:            "<1d",
		25:           "1d",
		24 * 29:      "29d",
		24 * 60:      "2mo",
		24 * 400:     "1y 1mo",
		24 * 365 * 5: "5y",
	}
	for hours, want := range cases {
		got := formatPowerOnAge(hours)
		if got != want {
			t.Errorf("formatPowerOnAge(%d) = %q, want %q", hours, got, want)
		}
	}
}

func TestSmartSectorCounts(t *testing.T) {
	attrs := []storage.SmartAttribute{
		{ID: "1", Raw: "0"},
		{ID: "5", Raw: "3"},
		{ID: "9", Raw: "100"},
		{ID: "197", Raw: "2"},
		{ID: "garbage", Raw: "x"},
	}
	r, p := smartSectorCounts(attrs)
	if r != 3 || p != 2 {
		t.Errorf("smartSectorCounts = (%d, %d), want (3, 2)", r, p)
	}
}

func TestDiskBayFallback(t *testing.T) {
	cases := []struct {
		disk storage.Disk
		want string
	}{
		{storage.Disk{SlotID: 4}, "4"},
		{storage.Disk{ID: "sda"}, "-"},
		{storage.Disk{ID: "nvme0n1"}, "1"},
		{storage.Disk{ID: ""}, "-"},
	}
	for _, c := range cases {
		got := diskBay(c.disk)
		if got != c.want {
			t.Errorf("diskBay(%+v) = %q, want %q", c.disk, got, c.want)
		}
	}
}

func TestRenderInfoHumanFlagsTemperatureWarn(t *testing.T) {
	dsm := &system.DSMInfo{Model: "X", VersionString: "DSM 7", Temperature: 88, TemperatureWarn: true}
	sys := &system.SystemInfo{}
	var buf bytes.Buffer
	renderInfoHuman(&buf, dsm, sys, nil, nil)
	out := buf.String()
	if !strings.Contains(out, "88 °C") {
		t.Fatalf("expected 88 °C in output: %s", out)
	}
	// PrintKVBlock with a non-TTY writer renders the badge as ASCII "WARN".
	if !strings.Contains(out, "WARN") {
		t.Fatalf("expected WARN badge in output: %s", out)
	}
}
