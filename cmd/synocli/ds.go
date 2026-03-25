package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

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
	cmd := &cobra.Command{Use: "add", Short: "Add downloads"}
	var destination string
	urlCmd := &cobra.Command{
		Use:   "url <endpoint> <url>",
		Short: "Add HTTP/HTTPS URL download",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			uri := args[1]
			return ac.withSession(cmd, endpoint, joinCommand("ds", "add", "url"), func(ctx context.Context, s *session) (any, error) {
				taskIDs, err := s.dsClient.AddURI(ctx, s.sid, uri, destination)
				if err != nil {
					return nil, err
				}
				data := map[string]any{"kind": "url", "uri": uri, "destination": destination, "task_ids": taskIDs}
				if ac.opts.JSON {
					return data, nil
				}
				if len(taskIDs) > 0 {
					fmt.Fprintf(ac.out, "added URL download: %s (task_ids=%s)\n", uri, strings.Join(taskIDs, ","))
				} else {
					fmt.Fprintf(ac.out, "added URL download: %s\n", uri)
				}
				return nil, nil
			})
		},
	}
	magnetCmd := &cobra.Command{
		Use:   "magnet <endpoint> <magnet>",
		Short: "Add magnet download",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			magnet := args[1]
			return ac.withSession(cmd, endpoint, joinCommand("ds", "add", "magnet"), func(ctx context.Context, s *session) (any, error) {
				taskIDs, err := s.dsClient.AddURI(ctx, s.sid, magnet, destination)
				if err != nil {
					return nil, err
				}
				data := map[string]any{"kind": "magnet", "uri": magnet, "destination": destination, "task_ids": taskIDs}
				if ac.opts.JSON {
					return data, nil
				}
				if len(taskIDs) > 0 {
					fmt.Fprintf(ac.out, "added magnet download (task_ids=%s)\n", strings.Join(taskIDs, ","))
				} else {
					fmt.Fprintln(ac.out, "added magnet download")
				}
				return nil, nil
			})
		},
	}
	torrentCmd := &cobra.Command{
		Use:   "torrent <endpoint> <file.torrent>",
		Short: "Add torrent file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpoint := args[0]
			torrentPath := args[1]
			if _, err := os.Stat(torrentPath); err != nil {
				return apperr.Wrap("validation_error", "invalid torrent file", 1, err)
			}
			return ac.withSession(cmd, endpoint, joinCommand("ds", "add", "torrent"), func(ctx context.Context, s *session) (any, error) {
				taskIDs, err := s.dsClient.AddTorrent(ctx, s.sid, torrentPath, destination)
				if err != nil {
					return nil, err
				}
				data := map[string]any{"kind": "torrent", "file": torrentPath, "destination": destination, "task_ids": taskIDs}
				if ac.opts.JSON {
					return data, nil
				}
				if len(taskIDs) > 0 {
					fmt.Fprintf(ac.out, "added torrent: %s (task_ids=%s)\n", torrentPath, strings.Join(taskIDs, ","))
				} else {
					fmt.Fprintf(ac.out, "added torrent: %s\n", torrentPath)
				}
				return nil, nil
			})
		},
	}
	urlCmd.Flags().StringVar(&destination, "destination", "", "Destination folder")
	magnetCmd.Flags().StringVar(&destination, "destination", "", "Destination folder")
	torrentCmd.Flags().StringVar(&destination, "destination", "", "Destination folder")
	cmd.AddCommand(urlCmd, magnetCmd, torrentCmd)
	return cmd
}

func newDSListCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "list <endpoint>",
		Short: "List downloads",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, args[0], joinCommand("ds", "list"), func(ctx context.Context, s *session) (any, error) {
				tasks, err := s.dsClient.List(ctx, s.sid)
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
						downloadstation.NormalizeStatus(t.Status),
						downloadstation.StatusDisplay(t.Status),
						t.Type,
						valueOrDash(destinationOf(t)),
						fmt.Sprintf("%d", t.Size),
						fmt.Sprintf("%d", downloadedOf(t)),
						fmt.Sprintf("%d", uploadedOf(t)),
						fmt.Sprintf("%d", downSpeedOf(t)),
						fmt.Sprintf("%d", upSpeedOf(t)),
					})
				}
				printTable(ac.out,
					[]string{"id", "title", "normalized", "raw", "type", "destination", "size", "downloaded", "uploaded", "down_speed", "up_speed"},
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
				t, err := s.dsClient.Get(ctx, s.sid, id)
				if err != nil {
					return nil, err
				}
				mapped := mapTask(*t)
				if ac.opts.JSON {
					return mapped, nil
				}
				for _, line := range []string{
					fmt.Sprintf("id: %s", t.ID),
					fmt.Sprintf("title: %s", t.Title),
					fmt.Sprintf("status: %s", downloadstation.StatusDisplay(t.Status)),
					fmt.Sprintf("type: %s", t.Type),
					fmt.Sprintf("destination: %s", valueOrDash(destinationOf(*t))),
					fmt.Sprintf("size: %d", t.Size),
					fmt.Sprintf("downloaded: %d", downloadedOf(*t)),
					fmt.Sprintf("uploaded: %d", uploadedOf(*t)),
					fmt.Sprintf("download_speed: %d", downSpeedOf(*t)),
					fmt.Sprintf("upload_speed: %d", upSpeedOf(*t)),
					fmt.Sprintf("uri: %s", valueOrDash(uriOf(*t))),
					fmt.Sprintf("status_extra: %s", valueOrDash(t.StatusExtra)),
				} {
					fmt.Fprintln(ac.out, line)
				}
				return nil, nil
			})
		},
	}
}

func newDSPauseCmd(ac *appContext) *cobra.Command {
	return actionWithIDs(ac, "pause", func(ctx context.Context, s *session, ids []string) error {
		return s.dsClient.Pause(ctx, s.sid, ids)
	})
}

func newDSResumeCmd(ac *appContext) *cobra.Command {
	return actionWithIDs(ac, "resume", func(ctx context.Context, s *session, ids []string) error {
		return s.dsClient.Resume(ctx, s.sid, ids)
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
				if err := s.dsClient.Delete(ctx, s.sid, ids, withData); err != nil {
					return nil, err
				}
				data := map[string]any{"task_ids": ids, "with_data": withData}
				if ac.opts.JSON {
					return data, nil
				}
				fmt.Fprintf(ac.out, "delete: %s\n", strings.Join(ids, ", "))
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
		Short: strings.Title(action) + " tasks",
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
				fmt.Fprintf(ac.out, "%s: %s\n", action, strings.Join(ids, ", "))
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
			endpoint, id := args[0], args[1]
			return ac.withSession(cmd, endpoint, joinCommand("ds", "wait"), func(ctx context.Context, s *session) (any, error) {
				deadline := time.Time{}
				if maxWait > 0 {
					deadline = time.Now().Add(maxWait)
				}
				for {
					task, err := s.dsClient.Get(ctx, s.sid, id)
					if err != nil {
						return nil, err
					}
					n := downloadstation.NormalizeStatus(task.Status)
					if downloadstation.IsTerminalSuccess(n) {
						data := map[string]any{"task": mapTask(*task), "result": "success"}
						if ac.opts.JSON {
							return data, nil
						}
						fmt.Fprintf(ac.out, "task %s completed with status %s\n", id, n)
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
			endpoint := args[0]
			start := time.Now()
			return ac.withSession(cmd, endpoint, joinCommand("ds", "watch"), func(ctx context.Context, s *session) (any, error) {
				statusSet := make(map[string]struct{}, len(statuses))
				for _, st := range statuses {
					statusSet[strings.ToLower(st)] = struct{}{}
				}
				idSet := make(map[string]struct{}, len(ids))
				for _, id := range ids {
					idSet[id] = struct{}{}
				}
				for {
					tasks, err := s.dsClient.List(ctx, s.sid)
					if err != nil {
						return nil, err
					}
					filtered := filterTasks(tasks, idSet, statusSet)
					if ac.opts.JSON {
						env := output.NewEnvelope(true, joinCommand("ds", "watch"), s.endpoint, start)
						env.Meta.APIVersion = s.apiVersions
						env.Data = map[string]any{"event": "snapshot", "tasks": mapTasks(filtered)}
						if err := output.WriteJSONLine(ac.out, env); err != nil {
							return nil, err
						}
					} else {
						fmt.Fprintf(ac.out, "[%s] tasks=%d\n", time.Now().Format(time.RFC3339), len(filtered))
						rows := make([][]string, 0, len(filtered))
						for _, t := range filtered {
							rows = append(rows, []string{t.ID, t.Title, downloadstation.NormalizeStatus(t.Status), downloadstation.StatusDisplay(t.Status), fmt.Sprintf("%d", downSpeedOf(t))})
						}
						printTable(ac.out, []string{"id", "title", "normalized", "raw", "down_speed"}, rows)
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
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Watch polling interval")
	cmd.Flags().StringSliceVar(&ids, "id", nil, "Filter by task ID")
	cmd.Flags().StringSliceVar(&statuses, "status", nil, "Filter by normalized status")
	return cmd
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

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
