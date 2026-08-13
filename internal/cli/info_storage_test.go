package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"synocli/internal/synology/storage"
)

func TestInfoStorageCommandRegistered(t *testing.T) {
	root, _ := newRootCmd(nil, &bytes.Buffer{}, &bytes.Buffer{})
	cmd, _, err := root.Find([]string{"info", "storage"})
	if err != nil || cmd.Name() != "storage" {
		t.Fatalf("info storage not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"watch", "interval"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing --%s flag", flag)
		}
	}
}

func TestFormatRAIDLevel(t *testing.T) {
	cases := []struct {
		deviceType, raidType, want string
	}{
		{"raid_5", "single", "RAID 5"},
		{"raid_10", "single", "RAID 10"},
		{"shr_1", "single", "SHR"},
		{"shr", "single", "SHR"},
		{"shr_2", "single", "SHR-2"},
		{"basic", "single", "Basic"},
		{"jbod", "single", "JBOD"},
		{"future_type", "single", "future_type"},
		{"", "single", "single"},
		{"", "", "-"},
	}
	for _, c := range cases {
		t.Run(c.deviceType+"/"+c.raidType, func(t *testing.T) {
			if got := formatRAIDLevel(c.deviceType, c.raidType); got != c.want {
				t.Errorf("formatRAIDLevel(%q, %q) = %q, want %q", c.deviceType, c.raidType, got, c.want)
			}
		})
	}
}

func TestFormatProgress(t *testing.T) {
	cases := []struct {
		name string
		p    storage.Progress
		want string
	}{
		{"active", storage.Progress{Percent: "7.10", Step: "raid_parity_checking"}, "7.10% raid_parity_checking"},
		{"no step", storage.Progress{Percent: "42.00"}, "42.00%"},
		{"idle empty", storage.Progress{}, "-"},
		{"idle minus one", storage.Progress{Percent: "-1", Step: "none"}, "-"},
		{"garbage", storage.Progress{Percent: "n/a"}, "-"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := formatProgress(c.p); got != c.want {
				t.Errorf("formatProgress(%+v) = %q, want %q", c.p, got, c.want)
			}
		})
	}
}

func TestVolumeStatusCell(t *testing.T) {
	v := storage.Volume{Status: "repairing", Progress: storage.Progress{Percent: "7.10", Step: "raid_parity_checking"}}
	if got := volumeStatusCell(v); got != "repairing 7.10%" {
		t.Errorf("cell = %q", got)
	}
	if got := volumeStatusCell(storage.Volume{Status: "normal"}); got != "normal" {
		t.Errorf("idle cell = %q", got)
	}
}

func rebuildFixture() *storage.LoadInfo {
	return &storage.LoadInfo{
		StoragePools: []storage.Pool{{
			ID:          "reuse_1",
			Status:      "repairing",
			RAIDType:    "single",
			DeviceType:  "raid_5",
			Disks:       []string{"sda", "sdb", "sdc", "sdd", "sde"},
			IsActioning: true,
			Progress:    storage.Progress{Percent: "7.10", Step: "raid_parity_checking"},
			Raids: []storage.Raid{{
				RaidPath:          "/dev/md2",
				HasParity:         true,
				NormalDevCount:    4,
				DesignedDiskCount: 5,
				Devices: []storage.RaidDevice{
					{ID: "sde", Slot: 4, Status: "normal"},
					{ID: "sdb", Slot: 1, Status: "rebuild"},
					{ID: "sda", Slot: 0, Status: "normal"},
				},
			}},
			Size: storage.Size{},
		}},
		Volumes: []storage.Volume{{
			ID: "volume_1", VolPath: "/volume1", Status: "repairing", FSType: "ext4",
			RAIDType: "single", DeviceType: "raid_5", IsActioning: true,
			Progress: storage.Progress{Percent: "7.10", Step: "raid_parity_checking"},
		}},
		Disks: []storage.Disk{
			{ID: "sda", Name: "Drive 1"},
			{ID: "sdb", Name: "Drive 2"},
			{ID: "sde", Name: "Drive 5"},
		},
	}
}

func TestRenderInfoStorageHumanRebuild(t *testing.T) {
	var buf bytes.Buffer
	renderInfoStorageHuman(&buf, rebuildFixture())
	out := buf.String()
	for _, want := range []string{
		"Storage Pools",
		"RAID 5",
		"repairing",
		"7.10% raid_parity_checking",
		"RAID /dev/md2 (pool reuse_1, 4/5 normal)",
		"rebuild",
		"Drive 2",
		"repairing 7.10%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Devices must be sorted by slot: sda (0) before sdb (1) before sde (4).
	if strings.Index(out, "Drive 1") > strings.Index(out, "Drive 2") ||
		strings.Index(out, "Drive 2") > strings.Index(out, "Drive 5") {
		t.Errorf("raid devices not sorted by slot:\n%s", out)
	}
}

func TestRenderInfoStorageHumanEmpty(t *testing.T) {
	var buf bytes.Buffer
	renderInfoStorageHuman(&buf, &storage.LoadInfo{})
	if !strings.Contains(buf.String(), "No storage pools or volumes reported.") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRebuildDynamics(t *testing.T) {
	base := time.Date(2026, 8, 13, 20, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		samples  []progressSample
		wantOK   bool
		wantRate float64
		wantETA  int64
	}{
		{"empty", nil, false, 0, 0},
		{"single sample", []progressSample{{base, 7.10}}, false, 0, 0},
		{
			// +0.5% over 1h -> 0.5%/h, 92.4% remaining -> 184.8h.
			"steady progress",
			[]progressSample{{base, 7.10}, {base.Add(30 * time.Minute), 7.35}, {base.Add(time.Hour), 7.60}},
			true, 0.5, int64(184.8 * 3600),
		},
		{"no progress", []progressSample{{base, 7.10}, {base.Add(time.Hour), 7.10}}, false, 0, 0},
		{"regressed", []progressSample{{base, 7.10}, {base.Add(time.Hour), 6.00}}, false, 0, 0},
		{"same timestamp", []progressSample{{base, 7.10}, {base, 7.20}}, false, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rate, eta, ok := rebuildDynamics(c.samples)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if diff := rate - c.wantRate; diff < -0.001 || diff > 0.001 {
				t.Errorf("rate = %v, want %v", rate, c.wantRate)
			}
			if diff := eta - c.wantETA; diff < -1 || diff > 1 {
				t.Errorf("eta = %v, want %v", eta, c.wantETA)
			}
		})
	}
}

func TestRenderRebuildDynamics(t *testing.T) {
	base := time.Date(2026, 8, 13, 20, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	renderRebuildDynamics(&buf, []progressSample{{base, 7.10}})
	if !strings.Contains(buf.String(), "Rebuild: 7.10%") || !strings.Contains(buf.String(), "ETA —") {
		t.Errorf("single-sample output = %q", buf.String())
	}
	buf.Reset()
	renderRebuildDynamics(&buf, []progressSample{{base, 7.10}, {base.Add(time.Hour), 7.60}})
	out := buf.String()
	if !strings.Contains(out, "Rebuild: 7.60%") || !strings.Contains(out, "+0.50%/h") || !strings.Contains(out, "ETA ≈") {
		t.Errorf("two-sample output = %q", out)
	}
	buf.Reset()
	renderRebuildDynamics(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("no-sample output = %q", buf.String())
	}
}

func TestStorageActioning(t *testing.T) {
	if !storageActioning(rebuildFixture()) {
		t.Error("rebuild fixture should be actioning")
	}
	idle := &storage.LoadInfo{
		StoragePools: []storage.Pool{{ID: "reuse_1", Status: "normal"}},
		Volumes:      []storage.Volume{{ID: "volume_1", Status: "normal"}},
	}
	if storageActioning(idle) {
		t.Error("idle fixture should not be actioning")
	}
}

func TestActiveRebuildPercent(t *testing.T) {
	if pct, ok := activeRebuildPercent(rebuildFixture()); !ok || pct != 7.10 {
		t.Errorf("pct = %v ok = %v", pct, ok)
	}
	// Falls back to volume progress when pools carry none (e.g. volume-level fsck).
	volOnly := &storage.LoadInfo{
		StoragePools: []storage.Pool{{ID: "reuse_1"}},
		Volumes:      []storage.Volume{{ID: "volume_1", Progress: storage.Progress{Percent: "12.50"}}},
	}
	if pct, ok := activeRebuildPercent(volOnly); !ok || pct != 12.50 {
		t.Errorf("volume fallback pct = %v ok = %v", pct, ok)
	}
	if _, ok := activeRebuildPercent(&storage.LoadInfo{}); ok {
		t.Error("empty store should have no percent")
	}
}
