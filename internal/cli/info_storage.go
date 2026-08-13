package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/cmdutil"
	"synocli/internal/output"
	"synocli/internal/synology/storage"
)

func newInfoStorageCmd(ac *appContext) *cobra.Command {
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Show storage pools, RAID members, and rebuild/scrub progress",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if watch {
				if err := validatePositiveDuration("--interval", interval); err != nil {
					return err
				}
			}
			// Declared outside the session closure so the auto re-login retry
			// on session expiry keeps the ETA history.
			var samples []progressSample
			return ac.withSession(cmd, joinCommand("info", "storage"), func(ctx context.Context, s *session) (any, error) {
				if watch {
					return nil, watchStorage(ctx, ac, s, interval, &samples)
				}
				store, err := s.storageClient.LoadInfo(ctx)
				if err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"storage": store}, nil
				}
				renderInfoStorageHuman(ac.out, store)
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&watch, "watch", false, "Continuous polling mode; exits when no action is in progress")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Polling interval")
	return cmd
}

// errWatchDone signals PollLoop to stop once no background action remains.
var errWatchDone = errors.New("storage watch complete")

func watchStorage(ctx context.Context, ac *appContext, s *session, interval time.Duration, samples *[]progressSample) error {
	ui := cmdutil.NewHumanUI(ac.out)
	err := cmdutil.PollLoop(ctx, interval, func() error {
		store, err := s.storageClient.LoadInfo(ctx)
		if err != nil {
			return err
		}
		now := time.Now()
		if pct, ok := activeRebuildPercent(store); ok {
			*samples = append(*samples, progressSample{at: now, pct: pct})
		}
		if ac.opts.JSON {
			env := output.NewEnvelope(true, joinCommand("info", "storage"), s.endpoint, s.start)
			env.Meta.APIVersion = s.apiVersions
			data := map[string]any{"event": "snapshot", "storage": store}
			if rate, eta, ok := rebuildDynamics(*samples); ok {
				data["rate_percent_per_hour"] = rate
				data["eta_seconds"] = eta
			}
			env.Data = data
			if werr := output.WriteJSONLine(ac.out, env); werr != nil {
				return werr
			}
		} else {
			if ui.Tty {
				_, _ = fmt.Fprint(ac.out, cmdutil.AnsiClearScreen)
			}
			_, _ = fmt.Fprintln(ac.out, ui.Muted(now.Format("2006-01-02 15:04:05")))
			renderInfoStorageHuman(ac.out, store)
			renderRebuildDynamics(ac.out, *samples)
			if !ui.Tty {
				_, _ = fmt.Fprintln(ac.out)
			}
		}
		if !storageActioning(store) {
			return errWatchDone
		}
		return nil
	})
	if errors.Is(err, errWatchDone) {
		return nil
	}
	return err
}

// progressSample is one (time, percent) observation of an active rebuild.
type progressSample struct {
	at  time.Time
	pct float64
}

// rebuildDynamics derives the average rate (%/hour) and remaining time from
// the first and last sample. ok is false until two samples with forward
// progress exist.
func rebuildDynamics(samples []progressSample) (ratePerHour float64, etaSeconds int64, ok bool) {
	if len(samples) < 2 {
		return 0, 0, false
	}
	first, last := samples[0], samples[len(samples)-1]
	dt := last.at.Sub(first.at).Seconds()
	if dt <= 0 {
		return 0, 0, false
	}
	ratePerSec := (last.pct - first.pct) / dt
	if ratePerSec <= 0 {
		return 0, 0, false
	}
	return ratePerSec * 3600, int64((100 - last.pct) / ratePerSec), true
}

func renderRebuildDynamics(w io.Writer, samples []progressSample) {
	if len(samples) == 0 {
		return
	}
	ui := cmdutil.NewHumanUI(w)
	last := samples[len(samples)-1]
	line := fmt.Sprintf("Rebuild: %.2f%%", last.pct)
	if rate, eta, ok := rebuildDynamics(samples); ok {
		line += fmt.Sprintf(" · +%.2f%%/h · ETA ≈ %s", rate, cmdutil.FormatDurationWords(eta))
	} else {
		line += " · ETA —"
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, ui.Muted(line))
}

// activeRebuildPercent returns the progress percent of the first pool (or
// volume) with a running background action.
func activeRebuildPercent(store *storage.LoadInfo) (float64, bool) {
	for _, p := range store.StoragePools {
		if pct, ok := parseProgressPercent(p.Progress); ok {
			return pct, true
		}
	}
	for _, v := range store.Volumes {
		if pct, ok := parseProgressPercent(v.Progress); ok {
			return pct, true
		}
	}
	return 0, false
}

func storageActioning(store *storage.LoadInfo) bool {
	for _, p := range store.StoragePools {
		if _, ok := parseProgressPercent(p.Progress); ok || p.IsActioning {
			return true
		}
	}
	for _, v := range store.Volumes {
		if _, ok := parseProgressPercent(v.Progress); ok || v.IsActioning {
			return true
		}
	}
	return false
}

// parseProgressPercent validates DSM's string percent ("7.10"). Idle
// pools/volumes report "" or "-1".
func parseProgressPercent(p storage.Progress) (float64, bool) {
	s := strings.TrimSpace(p.Percent)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

func renderInfoStorageHuman(w io.Writer, store *storage.LoadInfo) {
	ui := cmdutil.NewHumanUI(w)
	if store == nil || (len(store.StoragePools) == 0 && len(store.Volumes) == 0) {
		_, _ = fmt.Fprintln(w, ui.Muted("No storage pools or volumes reported."))
		return
	}
	driveNames := make(map[string]string, len(store.Disks))
	for _, d := range store.Disks {
		driveNames[d.ID] = d.Name
	}

	if len(store.StoragePools) > 0 {
		_, _ = fmt.Fprintln(w, ui.Title("Storage Pools"))
		rows := make([][]string, 0, len(store.StoragePools))
		for _, p := range store.StoragePools {
			rows = append(rows, []string{
				displayOrDash(p.ID),
				formatRAIDLevel(p.DeviceType, p.RAIDType),
				displayOrDash(p.Status),
				formatProgress(p.Progress),
				strconv.Itoa(len(p.Disks)),
				cmdutil.FormatBytes(p.Size.Total.Int64()),
			})
		}
		cmdutil.PrintTable(w, []string{"ID", "RAID", "STATUS", "PROGRESS", "DISKS", "TOTAL"}, rows)

		for _, p := range store.StoragePools {
			for _, r := range p.Raids {
				_, _ = fmt.Fprintln(w)
				title := fmt.Sprintf("RAID %s (pool %s, %d/%d normal)",
					displayOrDash(r.RaidPath), displayOrDash(p.ID), r.NormalDevCount, r.DesignedDiskCount)
				_, _ = fmt.Fprintln(w, ui.Title(title))
				devices := make([]storage.RaidDevice, len(r.Devices))
				copy(devices, r.Devices)
				sort.SliceStable(devices, func(i, j int) bool { return devices[i].Slot < devices[j].Slot })
				drows := make([][]string, 0, len(devices))
				for _, d := range devices {
					status := displayOrDash(d.Status)
					if d.Status != "" && d.Status != "normal" {
						status += " " + ui.Badge("warn")
					}
					drows = append(drows, []string{
						strconv.Itoa(d.Slot),
						displayOrDash(d.ID),
						displayOrDash(driveNames[d.ID]),
						status,
					})
				}
				cmdutil.PrintTable(w, []string{"SLOT", "DISK", "DRIVE", "STATUS"}, drows)
			}
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(store.Volumes) > 0 {
		printVolumes(w, store.Volumes)
	}
}

// formatRAIDLevel renders DSM's device_type ("raid_5", "shr_1", ...) as a
// human RAID level, falling back to the pool topology field (raidType) when
// device_type is absent (older DSM).
func formatRAIDLevel(deviceType, raidType string) string {
	dt := strings.TrimSpace(deviceType)
	switch {
	case dt == "":
		return displayOrDash(raidType)
	case strings.HasPrefix(dt, "raid_"):
		return "RAID " + strings.TrimPrefix(dt, "raid_")
	case dt == "shr_1" || dt == "shr":
		return "SHR"
	case dt == "shr_2":
		return "SHR-2"
	case dt == "basic":
		return "Basic"
	case dt == "jbod":
		return "JBOD"
	default:
		return dt
	}
}

// formatProgress renders "7.10% raid_parity_checking", or "-" when idle.
func formatProgress(p storage.Progress) string {
	if _, ok := parseProgressPercent(p); !ok {
		return "-"
	}
	out := strings.TrimSpace(p.Percent) + "%"
	if step := strings.TrimSpace(p.Step); step != "" {
		out += " " + step
	}
	return out
}

// volumeStatusCell appends the rebuild percent to the volume status, e.g.
// "repairing 7.10%".
func volumeStatusCell(v storage.Volume) string {
	st := displayOrDash(v.Status)
	if _, ok := parseProgressPercent(v.Progress); ok {
		st += " " + strings.TrimSpace(v.Progress.Percent) + "%"
	}
	return st
}
