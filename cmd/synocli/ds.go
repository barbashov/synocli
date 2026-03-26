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

	"synocli/internal/apperr"
	"synocli/internal/output"
	"synocli/internal/synology/downloadstation"
)

func newDSCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "ds", Aliases: []string{"downloadstation"}, Short: "Download Station commands"}
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
		Use:   "add <input>",
		Short: "Add download from URL, magnet, or torrent file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.TrimSpace(args[0])
			kind, err := detectAddInputKind(input)
			if err != nil {
				return apperr.Wrap("validation_error", "invalid add input", 1, err)
			}
			if kind == addInputTorrent {
				if err := downloadstation.ValidateTorrentFile(input); err != nil {
					return apperr.Wrap("validation_error", "invalid torrent file", 1, err)
				}
			}
			return ac.withSession(cmd, joinCommand("ds", "add"), func(ctx context.Context, s *session) (any, error) {
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
				inputKey := "URI"
				if kind == addInputTorrent {
					inputKey = "File"
				}
				printKVBlock(ac.out, "Download Added", []kvField{
					{Label: "Kind", Value: string(kind)},
					{Label: inputKey, Value: input},
					{Label: "Destination", Value: valueOrDash(destination)},
					{Label: "Task IDs", Value: joinOrDash(taskIDs)},
				})
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
		Use:   "list",
		Short: "List downloads",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("ds", "list"), func(ctx context.Context, s *session) (any, error) {
				tasks, err := s.dsClient.List(ctx)
				if err != nil {
					return nil, err
				}
				mapped := mapTasks(tasks)
				if ac.opts.JSON {
					return map[string]any{"tasks": mapped}, nil
				}
				printKVBlock(ac.out, "Downloads", []kvField{{Label: "Tasks", Value: fmt.Sprintf("%d", len(tasks))}})
				printTaskTable(ac.out, tasks)
				return nil, nil
			})
		},
	}
}

func newDSGetCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "get <task-id>",
		Short: "Get detailed task info",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			return ac.withSession(cmd, joinCommand("ds", "get"), func(ctx context.Context, s *session) (any, error) {
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
		Use:   "delete <task-id> [<task-id>...]",
		Short: "Delete tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := args
			return ac.withSession(cmd, joinCommand("ds", "delete"), func(ctx context.Context, s *session) (any, error) {
				if err := s.dsClient.Delete(ctx, ids, withData); err != nil {
					return nil, err
				}
				data := map[string]any{"task_ids": ids, "with_data": withData}
				if ac.opts.JSON {
					return data, nil
				}
				printKVBlock(ac.out, "Delete", []kvField{
					{Label: "Task IDs", Value: strings.Join(ids, ", ")},
					{Label: "With Data", Value: fmt.Sprintf("%t", withData)},
				})
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&withData, "with-data", false, "Delete task and downloaded data")
	return cmd
}

func actionWithIDs(ac *appContext, action string, run func(context.Context, *session, []string) error) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <task-id> [<task-id>...]", action),
		Short: strings.ToUpper(action[:1]) + action[1:] + " tasks",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := args
			return ac.withSession(cmd, joinCommand("ds", action), func(ctx context.Context, s *session) (any, error) {
				if err := run(ctx, s, ids); err != nil {
					return nil, err
				}
				data := map[string]any{"task_ids": ids}
				if ac.opts.JSON {
					return data, nil
				}
				printKVBlock(ac.out, strings.ToUpper(action[:1])+action[1:], []kvField{
					{Label: "Task IDs", Value: strings.Join(ids, ", ")},
				})
				return nil, nil
			})
		},
	}
}

func newDSWaitCmd(ac *appContext) *cobra.Command {
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   "wait <task-id>",
		Short: "Wait for a task to finish or fail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			id := args[0]
			return ac.withSession(cmd, joinCommand("ds", "wait"), func(ctx context.Context, s *session) (any, error) {
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
						ui := newHumanUI(ac.out)
						printKVBlock(ac.out, "Wait Result", []kvField{
							{Label: "Task ID", Value: id},
							{Label: "Result", Value: ui.status("success", "finished")},
							{Label: "Status", Value: ui.status(downloadstation.StatusDisplay(task.Status), n)},
						})
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
		Use:   "watch",
		Short: "Watch task state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withStreamingSession(cmd, joinCommand("ds", "watch"), func(ctx context.Context, s *session) error {
				ui := newHumanUI(ac.out)
				inPlace := ui.tty
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
						if inPlace {
							_, _ = fmt.Fprint(ac.out, ansiClearScreen)
						}
						printWatchHeader(ac.out, time.Now(), len(filtered), ids, statuses)
						printTaskTable(ac.out, filtered)
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
	ui := newHumanUI(w)
	printKVBlock(w, "Task Detail", []kvField{
		{Label: "ID", Value: t.ID},
		{Label: "Title", Value: t.Title},
		{Label: "Status", Value: ui.status(downloadstation.StatusDisplay(t.Status), downloadstation.NormalizeStatus(t.Status))},
		{Label: "Type", Value: t.Type},
		{Label: "Destination", Value: valueOrDash(destinationOf(t))},
		{Label: "Size", Value: formatBytes(t.Size)},
		{Label: "Downloaded", Value: formatBytes(downloadedOf(t))},
		{Label: "Progress", Value: formatPercent(downloadedOf(t), t.Size)},
		{Label: "Uploaded", Value: formatBytes(uploadedOf(t))},
		{Label: "Down Speed", Value: formatSpeed(downSpeedOf(t))},
		{Label: "Up Speed", Value: formatSpeed(upSpeedOf(t))},
		{Label: "URI", Value: valueOrDash(uriOf(t))},
		{Label: "Status Extra", Value: valueOrDash(t.StatusExtra)},
	})
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func printTaskTable(w io.Writer, tasks []downloadstation.Task) {
	ui := newHumanUI(w)
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{
			t.ID,
			t.Title,
			ui.status(downloadstation.StatusDisplay(t.Status), downloadstation.NormalizeStatus(t.Status)),
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
	printTable(w, []string{"ID", "Title", "Status", "Type", "Destination", "Size", "Downloaded", "Progress", "Uploaded", "Down Speed", "Up Speed"}, rows)
}
