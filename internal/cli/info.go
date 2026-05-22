package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

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
