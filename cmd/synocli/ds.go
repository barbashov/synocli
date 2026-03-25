package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/charmbracelet/lipgloss"

	"synocli/internal/apperr"
	"synocli/internal/output"
	"synocli/internal/synology/downloadstation"
)

func newDSCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "ds", Short: "Download Station commands"}
	cmd.AddCommand(
		newDSAddCmd(ac),
		newDSListCmd(ac),
		newDSGetCmd(ac),
		newDSPauseCmd(ac),
		newDSResumeCmd(ac),
		newDSDeleteCmd(ac),
		newDSWaitCmd(ac),
		newDSWatchCmd(ac),
	)
	return cmd
}

func newDSAddCmd(ac *appContext) *cobra.Command {
	var destination string
	cmd := &cobra.Command{
		Use:   "add <endpoint> <input>",
		Short: "Add download from URL, magnet, or torrent file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			input := strings.TrimSpace(args[1])
			kind, err := detectAddInputKind(input)
			if err != nil {
				return apperr.Wrap("validation_error", "invalid add input", 1, err)
			}
			if kind == addInputTorrent {
				if err := downloadstation.ValidateTorrentFile(input); err != nil {
					return apperr.Wrap("validation_error", "invalid torrent file", 1, err)
				}
			}
			return ac.withSession(cmd, endpoint, joinCommand("ds", "add"), func(ctx context.Context, s *session) (any, error) {
				var taskIDs []string
				switch kind {
				case addInputTorrent:
					taskIDs, err = s.dsClient.AddTorrent(ctx, input, destination)
				case addInputMagnet, addInputURL:
					taskIDs, err = s.dsClient.AddURI(ctx, input, destination)
				default:
					return nil, apperr.New("validation_error", "unsupported input type", 1)
				}
				if err != nil {
					return nil, err
				}
				data := map[string]any{"kind": string(kind), "destination": destination, "task_ids": taskIDs}
				if kind == addInputTorrent {
					data["file"] = input
				} else {
					data["uri"] = input
				}
				if ac.opts.JSON {
					return data, nil
				}
				switch kind {
				case addInputURL:
					if len(taskIDs) > 0 {
						_, _ = fmt.Fprintf(ac.out, "added URL download: %s (task_ids=%s)\n", input, strings.Join(taskIDs, ","))
					} else {
						_, _ = fmt.Fprintf(ac.out, "added URL download: %s\n", input)
					}
				case addInputMagnet:
					if len(taskIDs) > 0 {
						_, _ = fmt.Fprintf(ac.out, "added magnet download (task_ids=%s)\n", strings.Join(taskIDs, ","))
					} else {
						_, _ = fmt.Fprintln(ac.out, "added magnet download")
					}
				case addInputTorrent:
					if len(taskIDs) > 0 {
						_, _ = fmt.Fprintf(ac.out, "added torrent: %s (task_ids=%s)\n", input, strings.Join(taskIDs, ","))
					} else {
						_, _ = fmt.Fprintf(ac.out, "added torrent: %s\n", input)
					}
				}
				for _, tid := range taskIDs {
					task, err := s.dsClient.Get(ctx, tid)
					if err != nil {
						continue
					}
					_, _ = fmt.Fprintln(ac.out)
					printTaskDetail(ac.out, *task)
				}
				return nil, nil
			})
		},
	}
	cmd.Flags().StringVar(&destination, "destination", "", "Destination folder")
	return cmd
}

type addInputType string

const (
	addInputURL     addInputType = "url"
	addInputMagnet  addInputType = "magnet"
	addInputTorrent addInputType = "torrent"
)

func detectAddInputKind(input string) (addInputType, error) {
	lower := strings.ToLower(input)
	if strings.HasPrefix(lower, "magnet:") {
		return addInputMagnet, nil
	}
	st, err := os.Stat(input)
	if err == nil {
		if st.IsDir() {
			return "", fmt.Errorf("input %q is a directory, expected a torrent file", input)
		}
		return addInputTorrent, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat input: %w", err)
	}
	u, err := url.Parse(input)
	if err == nil && u.Scheme != "" && !strings.EqualFold(u.Scheme, "magnet") {
		return addInputURL, nil
	}
	return "", fmt.Errorf("cannot detect input type; expected magnet URI, existing torrent file path, or URL with scheme")
}

func newDSListCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "list <endpoint>",
		Short: "List downloads",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, args[0], joinCommand("ds", "list"), func(ctx context.Context, s *session) (any, error) {
				tasks, err := s.dsClient.List(ctx)
				if err != nil {
					return nil, err
				}
				mapped := mapTasks(tasks)
				if ac.opts.JSON {
					return map[string]any{"tasks": mapped}, nil
				}
				rows := make([][]string, 0, len(tasks))
				for _, t := range tasks {
					rows = append(rows, []string{
						t.ID,
						t.Title,
						downloadstation.StatusDisplay(t.Status),
						t.Type,
						valueOrDash(destinationOf(t)),
						formatBytes(t.Size),
						formatBytes(downloadedOf(t)),
						formatPercent(downloadedOf(t), t.Size),
						formatBytes(uploadedOf(t)),
						formatSpeed(downSpeedOf(t)),
						formatSpeed(upSpeedOf(t)),
					})
				}
				printTable(ac.out,
					[]string{"ID", "Title", "Status", "Type", "Destination", "Size", "Downloaded", "Progress", "Uploaded", "Down Speed", "Up Speed"},
					rows,
				)
				return nil, nil
			})
		},
	}
}

func newDSGetCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "get <endpoint> <task-id>",
		Short: "Get detailed task info",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			id := args[1]
			return ac.withSession(cmd, endpoint, joinCommand("ds", "get"), func(ctx context.Context, s *session) (any, error) {
				t, err := s.dsClient.Get(ctx, id)
				if err != nil {
					return nil, err
				}
				mapped := mapTask(*t)
				if ac.opts.JSON {
					return mapped, nil
				}
				printTaskDetail(ac.out, *t)
				return nil, nil
			})
		},
	}
}

func newDSPauseCmd(ac *appContext) *cobra.Command {
	return actionWithIDs(ac, "pause", func(ctx context.Context, s *session, ids []string) error {
		return s.dsClient.Pause(ctx, ids)
	})
}

func newDSResumeCmd(ac *appContext) *cobra.Command {
	return actionWithIDs(ac, "resume", func(ctx context.Context, s *session, ids []string) error {
		return s.dsClient.Resume(ctx, ids)
	})
}

func newDSDeleteCmd(ac *appContext) *cobra.Command {
	var withData bool
	cmd := &cobra.Command{
		Use:   "delete <endpoint> <task-id> [<task-id>...]",
		Short: "Delete tasks",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			ids := args[1:]
			return ac.withSession(cmd, endpoint, joinCommand("ds", "delete"), func(ctx context.Context, s *session) (any, error) {
				if err := s.dsClient.Delete(ctx, ids, withData); err != nil {
					return nil, err
				}
				data := map[string]any{"task_ids": ids, "with_data": withData}
				if ac.opts.JSON {
					return data, nil
				}
				_, _ = fmt.Fprintf(ac.out, "delete: %s\n", strings.Join(ids, ", "))
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&withData, "with-data", false, "Delete task and downloaded data")
	return cmd
}

func actionWithIDs(ac *appContext, action string, run func(context.Context, *session, []string) error) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <endpoint> <task-id> [<task-id>...]", action),
		Short: strings.ToUpper(action[:1]) + action[1:] + " tasks",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			ids := args[1:]
			return ac.withSession(cmd, endpoint, joinCommand("ds", action), func(ctx context.Context, s *session) (any, error) {
				if err := run(ctx, s, ids); err != nil {
					return nil, err
				}
				data := map[string]any{"task_ids": ids}
				if ac.opts.JSON {
					return data, nil
				}
				_, _ = fmt.Fprintf(ac.out, "%s: %s\n", action, strings.Join(ids, ", "))
				return nil, nil
			})
		},
	}
}

func newDSWaitCmd(ac *appContext) *cobra.Command {
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   "wait <endpoint> <task-id>",
		Short: "Wait for a task to finish or fail",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			endpoint, id := args[0], args[1]
			return ac.withSession(cmd, endpoint, joinCommand("ds", "wait"), func(ctx context.Context, s *session) (any, error) {
				deadline := time.Time{}
				if maxWait > 0 {
					deadline = time.Now().Add(maxWait)
				}
				for {
					task, err := s.dsClient.Get(ctx, id)
					if err != nil {
						return nil, err
					}
					n := downloadstation.NormalizeStatus(task.Status)
					if downloadstation.IsTerminalSuccess(n) {
						data := map[string]any{"task": mapTask(*task), "result": "success"}
						if ac.opts.JSON {
							return data, nil
						}
						_, _ = fmt.Fprintf(ac.out, "task %s completed with status %s\n", id, n)
						return nil, nil
					}
					if downloadstation.IsTerminalFailure(n) {
						return nil, &apperr.Error{Code: "task_failed", Message: fmt.Sprintf("task %s failed with status %s", id, task.Status), ExitCode: 4}
					}
					if !deadline.IsZero() && time.Now().After(deadline) {
						return nil, &apperr.Error{Code: "timeout", Message: fmt.Sprintf("timeout waiting for task %s", id), ExitCode: 5}
					}
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(interval):
					}
				}
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	return cmd
}

func newDSWatchCmd(ac *appContext) *cobra.Command {
	var interval time.Duration
	var ids []string
	var statuses []string
	cmd := &cobra.Command{
		Use:   "watch <endpoint>",
		Short: "Watch task state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			endpoint := args[0]
			return ac.withStreamingSession(cmd, endpoint, joinCommand("ds", "watch"), func(ctx context.Context, s *session) error {
				statusSet := make(map[string]struct{}, len(statuses))
				for _, st := range statuses {
					statusSet[strings.ToLower(st)] = struct{}{}
				}
				idSet := make(map[string]struct{}, len(ids))
				for _, id := range ids {
					idSet[id] = struct{}{}
				}
				for {
					tasks, err := s.dsClient.List(ctx)
					if err != nil {
						return err
					}
					filtered := filterTasks(tasks, idSet, statusSet)
					if ac.opts.JSON {
						env := output.NewEnvelope(true, joinCommand("ds", "watch"), s.endpoint, s.start)
						env.Meta.APIVersion = s.apiVersions
						env.Data = map[string]any{"event": "snapshot", "tasks": mapTasks(filtered)}
						if err := output.WriteJSONLine(ac.out, env); err != nil {
							return err
						}
					} else {
						_, _ = fmt.Fprintf(ac.out, "[%s] tasks=%d\n", time.Now().Format(time.RFC3339), len(filtered))
						rows := make([][]string, 0, len(filtered))
						for _, t := range filtered {
							rows = append(rows, []string{
								t.ID,
								t.Title,
								downloadstation.StatusDisplay(t.Status),
								t.Type,
								valueOrDash(destinationOf(t)),
								formatBytes(t.Size),
								formatBytes(downloadedOf(t)),
								formatPercent(downloadedOf(t), t.Size),
								formatBytes(uploadedOf(t)),
								formatSpeed(downSpeedOf(t)),
								formatSpeed(upSpeedOf(t)),
							})
						}
						printTable(ac.out, []string{"ID", "Title", "Status", "Type", "Destination", "Size", "Downloaded", "Progress", "Uploaded", "Down Speed", "Up Speed"}, rows)
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(interval):
					}
				}
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Watch polling interval")
	cmd.Flags().StringSliceVar(&ids, "id", nil, "Filter by task ID")
	cmd.Flags().StringSliceVar(&statuses, "status", nil, "Filter by normalized status")
	return cmd
}

func validatePositiveDuration(flagName string, value time.Duration) error {
	if value <= 0 {
		return apperr.New("validation_error", fmt.Sprintf("%s must be greater than 0", flagName), 1)
	}
	return nil
}

func filterTasks(tasks []downloadstation.Task, idSet, statusSet map[string]struct{}) []downloadstation.Task {
	out := make([]downloadstation.Task, 0, len(tasks))
	for _, t := range tasks {
		if len(idSet) > 0 {
			if _, ok := idSet[t.ID]; !ok {
				continue
			}
		}
		if len(statusSet) > 0 {
			if _, ok := statusSet[downloadstation.NormalizeStatus(t.Status)]; !ok {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

func mapTasks(tasks []downloadstation.Task) []map[string]any {
	out := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, mapTask(t))
	}
	return out
}

func mapTask(t downloadstation.Task) map[string]any {
	m := map[string]any{
		"task_id":           t.ID,
		"title":             t.Title,
		"normalized_status": downloadstation.NormalizeStatus(t.Status),
		"raw_status":        t.Status,
		"status_enum":       downloadstation.StatusEnum(t.Status),
		"status_display":    downloadstation.StatusDisplay(t.Status),
		"status_extra":      t.StatusExtra,
		"type":              t.Type,
		"username":          t.Username,
		"destination":       destinationOf(t),
		"uri":               uriOf(t),
		"size":              t.Size,
		"downloaded_size":   downloadedOf(t),
		"uploaded_size":     uploadedOf(t),
		"download_speed":    downSpeedOf(t),
		"upload_speed":      upSpeedOf(t),
		"created_time":      createdOf(t),
		"completed_time":    completedOf(t),
		"error_detail":      errorDetailOf(t),
		"tracker":           trackerOf(t),
		"peer":              peerOf(t),
		"file":              fileOf(t),
	}
	if code, ok := downloadstation.StatusCode(t.Status); ok {
		m["status_code"] = code
	}
	return m
}

func destinationOf(t downloadstation.Task) string {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.Destination
	}
	return ""
}

func uriOf(t downloadstation.Task) string {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.URI
	}
	return ""
}

func createdOf(t downloadstation.Task) int64 {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.CreateTime
	}
	return 0
}

func completedOf(t downloadstation.Task) int64 {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.CompletedTime
	}
	return 0
}

func errorDetailOf(t downloadstation.Task) string {
	if t.Additional != nil && t.Additional.Detail != nil {
		return t.Additional.Detail.ErrorDetail
	}
	return ""
}

func downloadedOf(t downloadstation.Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SizeDownloaded
	}
	return 0
}

func uploadedOf(t downloadstation.Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SizeUploaded
	}
	return 0
}

func downSpeedOf(t downloadstation.Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SpeedDownload
	}
	return 0
}

func upSpeedOf(t downloadstation.Task) int64 {
	if t.Additional != nil && t.Additional.Transfer != nil {
		return t.Additional.Transfer.SpeedUpload
	}
	return 0
}

func trackerOf(t downloadstation.Task) any {
	if t.Additional != nil {
		return t.Additional.Tracker
	}
	return nil
}

func peerOf(t downloadstation.Task) any {
	if t.Additional != nil {
		return t.Additional.Peer
	}
	return nil
}

func fileOf(t downloadstation.Task) any {
	if t.Additional != nil {
		return t.Additional.File
	}
	return nil
}

func printTaskDetail(w io.Writer, t downloadstation.Task) {
	label := lipgloss.NewStyle().Bold(true).Width(16)
	for _, line := range []string{
		label.Render("ID:") + t.ID,
		label.Render("Title:") + t.Title,
		label.Render("Status:") + downloadstation.StatusDisplay(t.Status),
		label.Render("Type:") + t.Type,
		label.Render("Destination:") + valueOrDash(destinationOf(t)),
		label.Render("Size:") + formatBytes(t.Size),
		label.Render("Downloaded:") + formatBytes(downloadedOf(t)),
		label.Render("Progress:") + formatPercent(downloadedOf(t), t.Size),
		label.Render("Uploaded:") + formatBytes(uploadedOf(t)),
		label.Render("Down Speed:") + formatSpeed(downSpeedOf(t)),
		label.Render("Up Speed:") + formatSpeed(upSpeedOf(t)),
		label.Render("URI:") + valueOrDash(uriOf(t)),
		label.Render("Status Extra:") + valueOrDash(t.StatusExtra),
	} {
		_, _ = fmt.Fprintln(w, line)
	}
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
