package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
	"synocli/internal/synology/storage"
	"synocli/internal/synology/system"
)

func newInfoCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show DSM system, hardware, storage, and health information",
		Args:  cobra.NoArgs,
		RunE:  runInfoOverview(ac),
	}
	cmd.AddCommand(newInfoUtilizationCmd(ac))
	cmd.AddCommand(newInfoDisksCmd(ac))
	return cmd
}

func runInfoOverview(ac *appContext) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return ac.withSession(cmd, "info", func(ctx context.Context, s *session) (any, error) {
			// Sequential: 4 LAN calls ~200ms; errgroup not justified.
			dsm, err := s.sysClient.GetDSMInfo(ctx)
			if err != nil {
				return nil, err
			}
			sys, err := s.sysClient.GetSystemInfo(ctx)
			if err != nil {
				return nil, err
			}
			// Best-effort: an account without storage-manager privilege
			// can still see DSM/system info.
			reboot, rebootErr := s.sysClient.GetNeedReboot(ctx)
			store, storeErr := s.storageClient.LoadInfo(ctx)

			if ac.opts.JSON {
				return overviewPayload(dsm, sys, reboot, rebootErr, store, storeErr), nil
			}
			renderInfoHuman(ac.out, dsm, sys, reboot, store)
			return nil, nil
		})
	}
}

func newInfoUtilizationCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "utilization",
		Short: "Show live CPU/RAM/disk/network load and diagnostic sensors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("info", "utilization"), func(ctx context.Context, s *session) (any, error) {
				util, err := s.sysClient.GetUtilization(ctx)
				if err != nil {
					return nil, err
				}
				// Temperature comes from DSM.Info; best-effort same as overview.
				dsm, dsmErr := s.sysClient.GetDSMInfo(ctx)
				status, statusErr := s.sysClient.GetSystemStatus(ctx)
				store, storeErr := s.storageClient.LoadInfo(ctx)

				if ac.opts.JSON {
					return utilizationPayload(util, dsm, dsmErr, status, statusErr, store, storeErr), nil
				}
				renderUtilizationHuman(ac.out, util, dsm, status, store)
				return nil, nil
			})
		},
	}
}

func overviewPayload(dsm *system.DSMInfo, sys *system.SystemInfo, reboot *system.NeedReboot, rebootErr error, store *storage.LoadInfo, storeErr error) map[string]any {
	out := map[string]any{"dsm": dsm, "system": sys}
	if reboot != nil {
		out["reboot"] = reboot
	}
	if rebootErr != nil {
		out["reboot_error"] = rebootErr.Error()
	}
	if store != nil {
		out["storage"] = store
	}
	if storeErr != nil {
		out["storage_error"] = storeErr.Error()
	}
	return out
}

func utilizationPayload(util *system.Utilization, dsm *system.DSMInfo, dsmErr error, status *system.SystemStatus, statusErr error, store *storage.LoadInfo, storeErr error) map[string]any {
	out := map[string]any{"utilization": util}
	if dsm != nil {
		out["dsm"] = dsm
	}
	if dsmErr != nil {
		out["dsm_error"] = dsmErr.Error()
	}
	if status != nil {
		out["status"] = status
	}
	if statusErr != nil {
		out["status_error"] = statusErr.Error()
	}
	if store != nil {
		out["storage"] = store
	}
	if storeErr != nil {
		out["storage_error"] = storeErr.Error()
	}
	return out
}

func renderInfoHuman(w io.Writer, dsm *system.DSMInfo, sys *system.SystemInfo, reboot *system.NeedReboot, store *storage.LoadInfo) {
	ui := cmdutil.NewHumanUI(w)

	cmdutil.PrintKVBlock(w, "System", []cmdutil.KVField{
		{Label: "Model", Value: dsm.Model},
		{Label: "Serial", Value: dsm.Serial},
		{Label: "DSM Version", Value: dsm.VersionString},
		{Label: "Uptime", Value: cmdutil.FormatDurationWords(dsm.Uptime)},
		{Label: "System Time", Value: displayOrDash(sys.Time)},
		{Label: "Time Zone", Value: timezoneDisplay(sys)},
	})
	_, _ = fmt.Fprintln(w)

	cmdutil.PrintKVBlock(w, "Hardware", []cmdutil.KVField{
		{Label: "CPU", Value: formatCPU(sys)},
		{Label: "RAM", Value: formatRAM(sys.RAMSize, dsm.RAM)},
		{Label: "Temperature", Value: formatTemp(ui, dsm.Temperature, dsm.TemperatureWarn)},
	})
	_, _ = fmt.Fprintln(w)

	if store != nil && len(store.Volumes) > 0 {
		printVolumes(w, store.Volumes)
		_, _ = fmt.Fprintln(w)
	}

	healthFields := []cmdutil.KVField{}
	if reboot != nil {
		healthFields = append(healthFields, cmdutil.KVField{Label: "Reboot Required", Value: yesNo(reboot.NeedReboot)})
	}
	if len(healthFields) > 0 {
		cmdutil.PrintKVBlock(w, "Health", healthFields)
	}
}

func renderUtilizationHuman(w io.Writer, util *system.Utilization, dsm *system.DSMInfo, status *system.SystemStatus, store *storage.LoadInfo) {
	ui := cmdutil.NewHumanUI(w)

	cmdutil.PrintKVBlock(w, "CPU", []cmdutil.KVField{
		{Label: "User", Value: pct(util.CPU.UserLoad)},
		{Label: "System", Value: pct(util.CPU.SystemLoad)},
		{Label: "Other", Value: pct(util.CPU.OtherLoad)},
		{Label: "Load Avg", Value: formatLoadAvg(util.CPU.Load1Min, util.CPU.Load5Min, util.CPU.Load15Min)},
	})
	_, _ = fmt.Fprintln(w)

	cmdutil.PrintKVBlock(w, "Memory", []cmdutil.KVField{
		{Label: "Real Usage", Value: formatMemUsage(util.Memory.RealUsage, util.Memory.TotalReal, util.Memory.AvailReal)},
		{Label: "Swap Usage", Value: formatSwapUsage(util.Memory.SwapUsage, util.Memory.TotalSwap)},
	})
	_, _ = fmt.Fprintln(w)

	if dsm != nil {
		cmdutil.PrintKVBlock(w, "Temperature", []cmdutil.KVField{
			{Label: "System", Value: formatTemp(ui, dsm.Temperature, dsm.TemperatureWarn)},
		})
		_, _ = fmt.Fprintln(w)
	}

	if len(util.Disk.Disks) > 0 {
		diskMeta := map[string]storage.Disk{}
		if store != nil {
			for _, d := range store.Disks {
				diskMeta[d.ID] = d
			}
		}
		printDiskUtilization(w, util.Disk.Disks, diskMeta)
		_, _ = fmt.Fprintln(w)
	}

	if len(util.Network) > 0 {
		printNetworkUtilization(w, util.Network)
		_, _ = fmt.Fprintln(w)
	}

	if status != nil {
		cmdutil.PrintKVBlock(w, "Status", []cmdutil.KVField{
			{Label: "System Crashed", Value: yesNo(status.IsSystemCrashed)},
			{Label: "Upgrade Ready", Value: yesNo(status.UpgradeReady)},
		})
	}
}

func timezoneDisplay(s *system.SystemInfo) string {
	if s.TimeZoneDesc != "" {
		return s.TimeZoneDesc
	}
	return displayOrDash(s.TimeZone)
}

func formatCPU(s *system.SystemInfo) string {
	parts := []string{}
	if s.CPUVendor != "" {
		parts = append(parts, s.CPUVendor)
	}
	if s.CPUFamily != "" {
		parts = append(parts, s.CPUFamily)
	}
	if s.CPUSeries != "" {
		parts = append(parts, s.CPUSeries)
	}
	base := strings.Join(parts, " ")
	suffix := []string{}
	if c := int(s.CPUCores); c > 0 {
		if c == 1 {
			suffix = append(suffix, "1 core")
		} else {
			suffix = append(suffix, fmt.Sprintf("%d cores", c))
		}
	}
	if s.CPUClockSpeed > 0 {
		suffix = append(suffix, fmt.Sprintf("@ %d MHz", s.CPUClockSpeed))
	}
	if len(suffix) > 0 {
		return base + ", " + strings.Join(suffix, " ")
	}
	return base
}

func formatRAM(sysRAMMB int64, dsmRAMMB int) string {
	mb := sysRAMMB
	if mb <= 0 {
		mb = int64(dsmRAMMB)
	}
	if mb <= 0 {
		return "-"
	}
	return cmdutil.FormatBytes(mb * 1024 * 1024)
}

func formatTemp(ui cmdutil.HumanUI, c int, warn bool) string {
	if c <= 0 {
		return "-"
	}
	v := fmt.Sprintf("%d °C", c)
	if warn {
		v = v + " " + ui.Badge("warn")
	}
	return v
}

func formatLoadAvg(a, b, c int) string {
	return fmt.Sprintf("%s  %s  %s  (1m / 5m / 15m)", loadDecimal(a), loadDecimal(b), loadDecimal(c))
}

// loadDecimal converts the centi-load values DSM returns (e.g. 106 -> 1.06).
func loadDecimal(v int) string {
	return fmt.Sprintf("%.2f", float64(v)/100.0)
}

func pct(v int) string { return strconv.Itoa(v) + "%" }

func formatMemUsage(usagePct int, totalKB, availKB int64) string {
	if totalKB <= 0 {
		return pct(usagePct)
	}
	return fmt.Sprintf("%s  (Total %s, Available %s)", pct(usagePct), cmdutil.FormatBytes(totalKB*1024), cmdutil.FormatBytes(availKB*1024))
}

func formatSwapUsage(usagePct int, totalKB int64) string {
	if totalKB <= 0 {
		return pct(usagePct)
	}
	return fmt.Sprintf("%s  (Total %s)", pct(usagePct), cmdutil.FormatBytes(totalKB*1024))
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func printVolumes(w io.Writer, vols []storage.Volume) {
	ui := cmdutil.NewHumanUI(w)
	_, _ = fmt.Fprintln(w, ui.Title("Volumes"))
	rows := make([][]string, 0, len(vols))
	for _, v := range vols {
		total := v.Size.Total.Int64()
		used := v.Size.Used.Int64()
		rows = append(rows, []string{
			displayOrDash(v.VolPath),
			displayOrDash(v.RAIDType),
			displayOrDash(v.FSType),
			cmdutil.FormatBytes(used),
			cmdutil.FormatBytes(total),
			cmdutil.FormatPercent(used, total),
			displayOrDash(v.Status),
		})
	}
	cmdutil.PrintTable(w, []string{"PATH", "RAID", "FS", "USED", "TOTAL", "USE%", "STATUS"}, rows)
}

func printDiskUtilization(w io.Writer, disks []system.DiskUtilization, meta map[string]storage.Disk) {
	ui := cmdutil.NewHumanUI(w)
	_, _ = fmt.Fprintln(w, ui.Title("Disks"))
	// Stable order: by display_name then device.
	sorted := make([]system.DiskUtilization, len(disks))
	copy(sorted, disks)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].DisplayName != sorted[j].DisplayName {
			return sorted[i].DisplayName < sorted[j].DisplayName
		}
		return sorted[i].Device < sorted[j].Device
	})
	rows := make([][]string, 0, len(sorted))
	for _, d := range sorted {
		md := meta[d.Device]
		rows = append(rows, []string{
			fallback(d.DisplayName, d.Device),
			displayOrDash(md.Model),
			diskTemp(md.Temp),
			displayOrDash(md.SmartStatus),
			pct(d.Utilization),
			cmdutil.FormatSpeed(d.ReadByte),
			cmdutil.FormatSpeed(d.WriteByte),
		})
	}
	cmdutil.PrintTable(w, []string{"NAME", "MODEL", "TEMP", "SMART", "UTIL%", "READ", "WRITE"}, rows)
}

func printNetworkUtilization(w io.Writer, nets []system.NetworkUsage) {
	ui := cmdutil.NewHumanUI(w)
	_, _ = fmt.Fprintln(w, ui.Title("Network"))
	rows := make([][]string, 0, len(nets))
	for _, n := range nets {
		// DSM reports a "total" pseudo-device first; keep it but flag it.
		rows = append(rows, []string{n.Device, cmdutil.FormatSpeed(n.RX), cmdutil.FormatSpeed(n.TX)})
	}
	cmdutil.PrintTable(w, []string{"INTERFACE", "RX", "TX"}, rows)
}

func diskTemp(c int) string {
	if c <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d °C", c)
}

func displayOrDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}

func fallback(primary, secondary string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return displayOrDash(secondary)
}

// --- info disks ------------------------------------------------------------

func newInfoDisksCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disks",
		Short: "Show per-bay drive state (model, type, age, temperature, SMART verdict)",
		Args:  cobra.NoArgs,
		RunE:  runInfoDisks(ac),
	}
	cmd.AddCommand(newInfoDisksSmartCmd(ac))
	return cmd
}

func runInfoDisks(ac *appContext) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		return ac.withSession(cmd, joinCommand("info", "disks"), func(ctx context.Context, s *session) (any, error) {
			store, err := s.storageClient.LoadInfo(ctx)
			if err != nil {
				return nil, err
			}
			disks := sortedDisks(store.Disks)
			rows := make([]diskRow, 0, len(disks))
			for _, d := range disks {
				row := diskRow{Disk: d}
				if d.Device != "" {
					hi, hErr := s.storageClient.GetHealthInfo(ctx, d.Device, d.ID)
					if hErr != nil {
						row.HealthError = hErr
					} else {
						row.Health = hi
					}
				}
				rows = append(rows, row)
			}

			if ac.opts.JSON {
				return diskOverviewPayload(store, rows), nil
			}
			renderInfoDisksHuman(ac.out, rows)
			return nil, nil
		})
	}
}

// diskRow pairs a disk with its (best-effort) SMART health response.
type diskRow struct {
	Disk        storage.Disk
	Health      *storage.HealthInfo
	HealthError error
}

func diskOverviewPayload(store *storage.LoadInfo, rows []diskRow) map[string]any {
	out := map[string]any{"storage": store}
	health := map[string]any{}
	for _, r := range rows {
		entry := map[string]any{}
		if r.Health != nil {
			entry["overview"] = r.Health.Overview
			ra, pe := smartSectorCounts(r.Health.SmartInfo)
			entry["reallocated"] = ra
			entry["pending"] = pe
		}
		if r.HealthError != nil {
			entry["error"] = r.HealthError.Error()
		}
		health[r.Disk.ID] = entry
	}
	out["health"] = health
	return out
}

func renderInfoDisksHuman(w io.Writer, rows []diskRow) {
	ui := cmdutil.NewHumanUI(w)
	if len(rows) == 0 {
		_, _ = fmt.Fprintln(w, ui.Muted("No disks reported."))
		return
	}
	_, _ = fmt.Fprintln(w, ui.Title("Disks"))
	table := make([][]string, 0, len(rows))
	healthy, warnings := 0, 0
	for _, r := range rows {
		health, warn := diskHealthCell(ui, r)
		if warn {
			warnings++
		} else {
			healthy++
		}
		table = append(table, []string{
			diskBay(r.Disk),
			fallback(r.Disk.Name, r.Disk.ID),
			displayOrDash(r.Disk.Model),
			cmdutil.FormatBytes(r.Disk.SizeTotal.Int64()),
			diskTypeCell(r.Disk),
			diskTemp(r.Disk.Temp),
			diskAgeCell(r),
			health,
			displayOrDash(r.Disk.UsedBy),
		})
	}
	cmdutil.PrintTable(w, []string{"BAY", "NAME", "MODEL", "SIZE", "TYPE", "TEMP", "AGE", "HEALTH", "USED BY"}, table)
	_, _ = fmt.Fprintln(w, ui.Muted(fmt.Sprintf("%s · %d healthy · %s", pluralize(len(rows), "disk", "disks"), healthy, pluralize(warnings, "warning", "warnings"))))
}

func sortedDisks(in []storage.Disk) []storage.Disk {
	out := make([]storage.Disk, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SlotID != out[j].SlotID {
			return out[i].SlotID < out[j].SlotID
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func diskBay(d storage.Disk) string {
	if d.SlotID > 0 {
		return strconv.Itoa(d.SlotID)
	}
	// fallback: last numeric run in the id (e.g. "sda" -> "-", "nvme0" -> "0")
	for i := len(d.ID) - 1; i >= 0; i-- {
		if d.ID[i] < '0' || d.ID[i] > '9' {
			if i == len(d.ID)-1 {
				return "-"
			}
			return d.ID[i+1:]
		}
	}
	if d.ID == "" {
		return "-"
	}
	return d.ID
}

func diskTypeCell(d storage.Disk) string {
	// Prefer connector kind when DSM signals a non-SATA bus (USB, eSATA).
	bus := strings.TrimSpace(d.DiskType)
	if bus != "" && !strings.EqualFold(bus, "SATA") {
		return bus
	}
	if d.IsSSD {
		if d.RemainLife >= 0 && d.RemainLife <= 100 {
			return fmt.Sprintf("SSD %d%%", d.RemainLife)
		}
		return "SSD"
	}
	return "HDD"
}

func diskAgeCell(r diskRow) string {
	if r.HealthError != nil {
		return "-"
	}
	if r.Health == nil {
		return "-"
	}
	hours := r.Health.Overview.Poweron.Int64()
	return formatPowerOnAge(hours)
}

// formatPowerOnAge renders a power-on-hours count as a short human age.
// Output examples: "-" (0), "<1d" (<24h), "5d", "8mo", "5y 5mo".
func formatPowerOnAge(hours int64) string {
	if hours <= 0 {
		return "-"
	}
	if hours < 24 {
		return "<1d"
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	years := months / 12
	rem := months % 12
	if rem == 0 {
		return fmt.Sprintf("%dy", years)
	}
	return fmt.Sprintf("%dy %dmo", years, rem)
}

// diskHealthCell renders the HEALTH column and reports whether this disk
// should count toward the "warnings" footer. The cell combines DSM's curated
// flags (smart status, exceed_bad_sector_thr, below_remain_life_thr,
// read_only) with raw SMART id 5 (Reallocated_Sector_Ct) and id 197
// (Current_Pending_Sector), so sub-threshold issues surface early.
func diskHealthCell(ui cmdutil.HumanUI, r diskRow) (string, bool) {
	if r.HealthError != nil {
		return "error " + ui.Badge("warn"), true
	}
	reasons := []string{}
	warn := false

	smartStatus := "normal"
	if r.Health != nil && strings.TrimSpace(r.Health.Overview.Smart) != "" {
		smartStatus = r.Health.Overview.Smart
	} else if strings.TrimSpace(r.Disk.SmartStatus) != "" {
		smartStatus = r.Disk.SmartStatus
	}
	if smartStatus != "normal" {
		reasons = append(reasons, smartStatus)
		warn = true
	}

	var reallocated, pending int64
	if r.Health != nil {
		reallocated, pending = smartSectorCounts(r.Health.SmartInfo)
	}
	if r.Disk.ExceedBadSectorThr || reallocated > 0 {
		reasons = append(reasons, fmt.Sprintf("%d bad", reallocated))
		if r.Disk.ExceedBadSectorThr {
			warn = true
		}
	}
	uncCount := int64(0)
	if r.Health != nil {
		uncCount = int64(r.Health.Overview.UNC)
	}
	if uncCount > 0 || pending > 0 {
		// Show whichever count is higher; both describe "uncorrectable / pending sectors".
		if pending > uncCount {
			reasons = append(reasons, fmt.Sprintf("%d pending", pending))
		} else {
			reasons = append(reasons, fmt.Sprintf("%d pending", uncCount))
		}
		warn = true
	}

	if r.Disk.BelowRemainLifeThr || (r.Disk.IsSSD && r.Disk.RemainLife >= 0 && r.Disk.RemainLife < 20) {
		life := r.Disk.RemainLife
		if life < 0 && r.Health != nil {
			life = r.Health.Overview.RemainLife
		}
		if life >= 0 {
			reasons = append(reasons, fmt.Sprintf("%d%% life", life))
		} else {
			reasons = append(reasons, "life-low")
		}
		warn = true
	}

	if r.Health != nil && r.Health.Overview.ReadOnly {
		reasons = append(reasons, "read-only")
		warn = true
	}

	if len(reasons) == 0 {
		return "normal", false
	}
	cell := strings.Join(reasons, ", ")
	if warn {
		cell = cell + " " + ui.Badge("warn")
	}
	return cell, warn
}

// smartSectorCounts extracts raw Reallocated_Sector_Ct (id 5) and
// Current_Pending_Sector (id 197) counts from a SMART attribute table.
// Malformed/missing rows decode as 0.
func smartSectorCounts(attrs []storage.SmartAttribute) (reallocated, pending int64) {
	for _, a := range attrs {
		switch a.ID {
		case "5":
			reallocated = parseInt64(a.Raw)
		case "197":
			pending = parseInt64(a.Raw)
		}
	}
	return reallocated, pending
}

func parseInt64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

// --- info disks smart ------------------------------------------------------

func newInfoDisksSmartCmd(ac *appContext) *cobra.Command {
	var diskFilter string
	cmd := &cobra.Command{
		Use:   "smart",
		Short: "Show detailed SMART attributes for each drive",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("info", "disks", "smart"), func(ctx context.Context, s *session) (any, error) {
				store, err := s.storageClient.LoadInfo(ctx)
				if err != nil {
					return nil, err
				}
				disks := sortedDisks(store.Disks)
				filter := strings.TrimSpace(diskFilter)
				if filter != "" {
					found := false
					for _, d := range disks {
						if d.ID == filter {
							disks = []storage.Disk{d}
							found = true
							break
						}
					}
					if !found {
						return nil, apperr.New("validation_error", fmt.Sprintf("disk %q not found", filter), 1)
					}
				}

				rows := make([]diskRow, 0, len(disks))
				for _, d := range disks {
					row := diskRow{Disk: d}
					if d.Device != "" {
						hi, hErr := s.storageClient.GetHealthInfo(ctx, d.Device, d.ID)
						if hErr != nil {
							row.HealthError = hErr
						} else {
							row.Health = hi
						}
					}
					rows = append(rows, row)
				}

				if ac.opts.JSON {
					return diskSmartPayload(rows), nil
				}
				renderInfoDisksSmartHuman(ac.out, rows)
				return nil, nil
			})
		},
	}
	cmd.Flags().StringVar(&diskFilter, "disk", "", "Limit to a single disk by id (e.g. sda)")
	return cmd
}

func diskSmartPayload(rows []diskRow) map[string]any {
	out := map[string]any{}
	health := map[string]any{}
	for _, r := range rows {
		entry := map[string]any{}
		if r.Health != nil {
			entry["overview"] = r.Health.Overview
			entry["smart_info"] = r.Health.SmartInfo
			ra, pe := smartSectorCounts(r.Health.SmartInfo)
			entry["reallocated"] = ra
			entry["pending"] = pe
		}
		if r.HealthError != nil {
			entry["error"] = r.HealthError.Error()
		}
		health[r.Disk.ID] = entry
	}
	out["health"] = health
	return out
}

func renderInfoDisksSmartHuman(w io.Writer, rows []diskRow) {
	ui := cmdutil.NewHumanUI(w)
	if len(rows) == 0 {
		_, _ = fmt.Fprintln(w, ui.Muted("No disks reported."))
		return
	}
	for i, r := range rows {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}
		title := fmt.Sprintf("Disk: %s (%s)", fallback(r.Disk.Name, r.Disk.ID), r.Disk.ID)
		_, _ = fmt.Fprintln(w, ui.Title(title))
		if r.HealthError != nil {
			cmdutil.PrintError(w, r.HealthError)
			continue
		}
		fields := []cmdutil.KVField{
			{Label: "Model", Value: displayOrDash(r.Disk.Model)},
			{Label: "Serial", Value: displayOrDash(r.Disk.Serial)},
			{Label: "Firmware", Value: displayOrDash(r.Disk.Firm)},
			{Label: "Type", Value: diskTypeCell(r.Disk)},
			{Label: "Size", Value: cmdutil.FormatBytes(r.Disk.SizeTotal.Int64())},
			{Label: "Used By", Value: displayOrDash(r.Disk.UsedBy)},
		}
		if r.Health != nil {
			ov := r.Health.Overview
			smartCell := displayOrDash(ov.Smart)
			if r.Disk.ExceedBadSectorThr || r.Disk.BelowRemainLifeThr || ov.UNC > 0 {
				smartCell = smartCell + " " + ui.Badge("warn")
			}
			fields = append(fields,
				cmdutil.KVField{Label: "SMART Status", Value: smartCell},
				cmdutil.KVField{Label: "SMART Test", Value: displayOrDash(ov.SmartTest)},
				cmdutil.KVField{Label: "Power On", Value: formatPowerOn(ov.Poweron.Int64())},
				cmdutil.KVField{Label: "IDNF / Retry / UNC", Value: fmt.Sprintf("%d / %d / %d", ov.IDNF, ov.Retry, ov.UNC)},
				cmdutil.KVField{Label: "Remain Life", Value: formatRemainLife(r.Disk.RemainLife, ov.RemainLife)},
			)
			if next := nextSmartTests(ov.SmartScheduleList); next != "" {
				fields = append(fields, cmdutil.KVField{Label: "Next Tests", Value: next})
			}
		}
		cmdutil.PrintKVBlock(w, "", fields)

		if r.Health != nil && len(r.Health.SmartInfo) > 0 {
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintln(w, ui.Title("Attributes"))
			attrRows := make([][]string, 0, len(r.Health.SmartInfo))
			for _, a := range r.Health.SmartInfo {
				attrRows = append(attrRows, []string{
					displayOrDash(a.ID),
					displayOrDash(a.Name),
					displayOrDash(a.Current),
					displayOrDash(a.Worst),
					displayOrDash(a.Threshold),
					displayOrDash(a.Raw),
					displayOrDash(a.Status),
				})
			}
			cmdutil.PrintTable(w, []string{"ID", "NAME", "CURRENT", "WORST", "THRESHOLD", "RAW", "STATUS"}, attrRows)
		}
	}
}

func formatPowerOn(hours int64) string {
	if hours <= 0 {
		return "-"
	}
	age := formatPowerOnAge(hours)
	if age == "-" || age == "<1d" {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh (%s)", hours, age)
}

func formatRemainLife(diskVal, overviewVal int) string {
	v := diskVal
	if v < 0 {
		v = overviewVal
	}
	if v < 0 {
		return "-"
	}
	return fmt.Sprintf("%d%%", v)
}

func nextSmartTests(entries []storage.SmartScheduleEntry) string {
	parts := []string{}
	for _, e := range entries {
		s := strings.TrimSpace(e.NextTriggerTime)
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}
