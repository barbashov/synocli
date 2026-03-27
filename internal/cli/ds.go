package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
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
		newDSCleanupCmd(ac),
		newDSWaitCmd(ac),
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
			kind, err := downloadstation.DetectAddInputKind(input)
			if err != nil {
				return apperr.Wrap("validation_error", "invalid add input", 1, err)
			}
			if kind == downloadstation.AddInputTorrent {
				if err := downloadstation.ValidateTorrentFile(input); err != nil {
					return apperr.Wrap("validation_error", "invalid torrent file", 1, err)
				}
			}
			return ac.withSession(cmd, joinCommand("ds", "add"), func(ctx context.Context, s *session) (any, error) {
				var taskIDs []string
				switch kind {
				case downloadstation.AddInputTorrent:
					taskIDs, err = s.dsClient.AddTorrent(ctx, input, destination)
				case downloadstation.AddInputMagnet, downloadstation.AddInputURL:
					taskIDs, err = s.dsClient.AddURI(ctx, input, destination)
				default:
					return nil, apperr.New("validation_error", "unsupported input type", 1)
				}
				if err != nil {
					return nil, err
				}
				data := map[string]any{"kind": string(kind), "destination": destination, "task_ids": taskIDs}
				if kind == downloadstation.AddInputTorrent {
					data["file"] = input
				} else {
					data["uri"] = input
				}
				if ac.opts.JSON {
					return data, nil
				}
				inputKey := "URI"
				if kind == downloadstation.AddInputTorrent {
					inputKey = "File"
				}
				cmdutil.PrintKVBlock(ac.out, "Download Added", []cmdutil.KVField{
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
	cmd.Flags().StringVar(&destination, "to", "", "Destination folder")
	return cmd
}

func newDSListCmd(ac *appContext) *cobra.Command {
	var ids []string
	var statuses []string
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List downloads",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if watch {
				if err := validatePositiveDuration("--interval", interval); err != nil {
					return err
				}
			}
			return ac.withSession(cmd, joinCommand("ds", "list"), func(ctx context.Context, s *session) (any, error) {
				statusSet := make(map[string]struct{}, len(statuses))
				for _, st := range statuses {
					statusSet[strings.ToLower(st)] = struct{}{}
				}
				idSet := make(map[string]struct{}, len(ids))
				for _, id := range ids {
					idSet[id] = struct{}{}
				}
				fetch := func() ([]downloadstation.Task, error) {
					tasks, err := s.dsClient.List(ctx)
					if err != nil {
						return nil, err
					}
					return downloadstation.FilterTasks(tasks, idSet, statusSet), nil
				}
				if watch {
					ui := cmdutil.NewHumanUI(ac.out)
					return nil, cmdutil.PollLoop(ctx, interval, func() error {
						filtered, err := fetch()
						if err != nil {
							return err
						}
						if ac.opts.JSON {
							env := output.NewEnvelope(true, joinCommand("ds", "list"), s.endpoint, s.start)
							env.Meta.APIVersion = s.apiVersions
							env.Data = map[string]any{"event": "snapshot", "tasks": downloadstation.MapTasks(filtered)}
							return output.WriteJSONLine(ac.out, env)
						}
						if ui.Tty {
							_, _ = fmt.Fprint(ac.out, cmdutil.AnsiClearScreen)
						}
						cmdutil.PrintWatchHeader(ac.out, time.Now(), len(filtered), ids, statuses)
						printTaskTable(ac.out, filtered)
						return nil
					})
				}
				filtered, err := fetch()
				if err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"tasks": downloadstation.MapTasks(filtered)}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Downloads", []cmdutil.KVField{{Label: "Tasks", Value: fmt.Sprintf("%d", len(filtered))}})
				printTaskTable(ac.out, filtered)
				return nil, nil
			})
		},
	}
	cmd.Flags().StringSliceVar(&ids, "id", nil, "Filter by task ID")
	cmd.Flags().StringSliceVar(&statuses, "status", nil, "Filter by normalized status")
	cmd.Flags().BoolVar(&watch, "watch", false, "Continuous polling mode")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	return cmd
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
				mapped := downloadstation.MapTask(*t)
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
	cmd := actionWithIDs(ac, "delete", func(ctx context.Context, s *session, ids []string) error {
		return s.dsClient.Delete(ctx, ids)
	})
	cmd.Aliases = []string{"rm"}
	return cmd
}

func newDSCleanupCmd(ac *appContext) *cobra.Command {
	var includeSeeding bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete finished downloads while keeping data intact",
		Long:  "Delete finished downloads while keeping data intact.\n\nIn JSON mode (--json) the confirmation prompt is skipped automatically.\nUse --yes (-y) to skip confirmation in interactive mode.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("ds", "cleanup"), func(ctx context.Context, s *session) (any, error) {
				tasks, err := s.dsClient.List(ctx)
				if err != nil {
					return nil, err
				}
				statuses := cleanupStatuses(includeSeeding)
				matchedTasks := cleanupTasks(tasks, statuses)
				ids := cleanupTaskIDs(matchedTasks)
				base := map[string]any{
					"include_seeding":  cleanupIncludesSeeding(statuses),
					"matched_count":    len(ids),
					"matched_task_ids": ids,
					"data_kept_intact": true,
				}
				if len(ids) == 0 {
					base["deleted_count"] = 0
					base["failed_count"] = 0
					base["deleted_task_ids"] = []string{}
					base["failed_task_ids"] = []string{}
					base["failed_tasks"] = []map[string]any{}
					if ac.opts.JSON {
						return base, nil
					}
					_, _ = fmt.Fprintln(ac.out, "Nothing to cleanup")
					return nil, nil
				}
				if !ac.opts.JSON && !yes {
					confirmed, err := promptCleanupConfirmation(ac.stdin, ac.out, matchedTasks, statuses)
					if err != nil {
						return nil, err
					}
					if !confirmed {
						return nil, apperr.New("cancelled", "cleanup cancelled", 1)
					}
				}
				if err := s.dsClient.Delete(ctx, ids); err != nil {
					var apiErr *downloadstation.APIError
					if errors.As(err, &apiErr) && len(apiErr.FailedTasks) > 0 {
						failedTasks := make([]map[string]any, 0, len(apiErr.FailedTasks))
						failedSet := make(map[string]struct{}, len(apiErr.FailedTasks))
						failedIDs := make([]string, 0, len(apiErr.FailedTasks))
						for _, ft := range apiErr.FailedTasks {
							failedTasks = append(failedTasks, map[string]any{"id": ft.ID, "code": ft.Code})
							for _, id := range normalizeFailedTaskIDs(ft.ID) {
								if _, ok := failedSet[id]; !ok {
									failedSet[id] = struct{}{}
									failedIDs = append(failedIDs, id)
								}
							}
						}
						deletedIDs := make([]string, 0, len(ids))
						for _, id := range ids {
							if _, failed := failedSet[id]; !failed {
								deletedIDs = append(deletedIDs, id)
							}
						}
						details := map[string]any{
							"include_seeding":  cleanupIncludesSeeding(statuses),
							"matched_count":    len(ids),
							"deleted_count":    len(deletedIDs),
							"failed_count":     len(failedTasks),
							"matched_task_ids": ids,
							"deleted_task_ids": deletedIDs,
							"failed_task_ids":  failedIDs,
							"failed_tasks":     failedTasks,
							"data_kept_intact": true,
						}
						if !ac.opts.JSON {
							printCleanupSummary(ac.out, statuses, len(ids), len(deletedIDs), len(failedTasks))
						}
						return nil, &apperr.Error{
							Code:     "partial_failure",
							Message:  "cleanup completed with partial failures",
							ExitCode: 1,
							Details:  details,
							Err:      err,
						}
					}
					return nil, err
				}
				base["deleted_count"] = len(ids)
				base["failed_count"] = 0
				base["deleted_task_ids"] = ids
				base["failed_task_ids"] = []string{}
				base["failed_tasks"] = []map[string]any{}
				if ac.opts.JSON {
					return base, nil
				}
				printCleanupSummary(ac.out, statuses, len(ids), len(ids), 0)
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVarP(&includeSeeding, "include-seeding", "s", false, "Also cleanup seeding tasks")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt")
	return cmd
}

func actionWithIDs(ac *appContext, action string, run func(context.Context, *session, []string) error) *cobra.Command {
	return &cobra.Command{
		Use:   fmt.Sprintf("%s <task-id> [<task-id>...]", action),
		Short: capitalizeWord(action) + " tasks",
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
				cmdutil.PrintKVBlock(ac.out, capitalizeWord(action), []cmdutil.KVField{
					{Label: "Task IDs", Value: strings.Join(ids, ", ")},
				})
				return nil, nil
			})
		},
	}
}

func cleanupStatuses(includeSeeding bool) map[string]struct{} {
	out := map[string]struct{}{"finished": {}}
	if includeSeeding {
		out["seeding"] = struct{}{}
	}
	return out
}

func cleanupIncludesSeeding(statusSet map[string]struct{}) bool {
	_, ok := statusSet["seeding"]
	return ok
}

func cleanupTasks(tasks []downloadstation.Task, statusSet map[string]struct{}) []downloadstation.Task {
	out := make([]downloadstation.Task, 0, len(tasks))
	for _, task := range tasks {
		if _, ok := statusSet[downloadstation.NormalizeStatus(task.Status)]; ok {
			out = append(out, task)
		}
	}
	return out
}

func cleanupTaskIDs(tasks []downloadstation.Task) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.ID)
	}
	return out
}

func normalizeFailedTaskIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err == nil {
			out := make([]string, 0, len(ids))
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id != "" {
					out = append(out, id)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func promptCleanupConfirmation(in io.Reader, out io.Writer, tasks []downloadstation.Task, statusSet map[string]struct{}) (bool, error) {
	if !isTTYReader(in) {
		return false, apperr.New("validation_error", "cleanup confirmation requires an interactive terminal; pass --yes (-y) to skip prompt", 1)
	}
	printCleanupPreview(out, tasks, statusSet)
	statusList := "finished"
	if _, ok := statusSet["seeding"]; ok {
		statusList = "finished, seeding"
	}
	_, _ = fmt.Fprintf(out, "Cleanup will delete %d Download Station task(s) with status: %s.\n", len(tasks), statusList)
	_, _ = fmt.Fprintln(out, "Downloaded data/files will be kept intact.")
	_, _ = fmt.Fprint(out, "Proceed? [y/N]: ")
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, apperr.Wrap("internal_error", "read confirmation input", 1, err)
	}
	return isAffirmativeAnswer(line), nil
}

func isAffirmativeAnswer(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func isTTYReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func printCleanupSummary(w io.Writer, statusSet map[string]struct{}, matched, deleted, failed int) {
	statusList := "finished"
	if _, ok := statusSet["seeding"]; ok {
		statusList = "finished, seeding"
	}
	cmdutil.PrintKVBlock(w, "Cleanup", []cmdutil.KVField{
		{Label: "Statuses", Value: statusList},
		{Label: "Matched", Value: fmt.Sprintf("%d", matched)},
		{Label: "Deleted", Value: fmt.Sprintf("%d", deleted)},
		{Label: "Failed", Value: fmt.Sprintf("%d", failed)},
		{Label: "Data", Value: "kept intact"},
	})
}

func printCleanupPreview(w io.Writer, tasks []downloadstation.Task, statusSet map[string]struct{}) {
	statusFilter := "finished"
	if _, ok := statusSet["seeding"]; ok {
		statusFilter = "finished,seeding"
	}
	cmdutil.PrintKVBlock(w, "Downloads", []cmdutil.KVField{
		{Label: "Tasks", Value: fmt.Sprintf("%d", len(tasks))},
		{Label: "Status Filter", Value: statusFilter},
	})
	printTaskTable(w, tasks)
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
						data := map[string]any{"task": downloadstation.MapTask(*task), "result": "success"}
						if ac.opts.JSON {
							return data, nil
						}
						ui := cmdutil.NewHumanUI(ac.out)
						cmdutil.PrintKVBlock(ac.out, "Wait Result", []cmdutil.KVField{
							{Label: "Task ID", Value: id},
							{Label: "Result", Value: ui.Status("success", "finished")},
							{Label: "Status", Value: ui.Status(downloadstation.StatusDisplay(task.Status), n)},
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

func validatePositiveDuration(flagName string, value time.Duration) error {
	if value <= 0 {
		return apperr.New("validation_error", fmt.Sprintf("%s must be greater than 0", flagName), 1)
	}
	return nil
}

func printTaskDetail(w io.Writer, t downloadstation.Task) {
	ui := cmdutil.NewHumanUI(w)
	cmdutil.PrintKVBlock(w, "Task Detail", []cmdutil.KVField{
		{Label: "ID", Value: t.ID},
		{Label: "Title", Value: t.Title},
		{Label: "Status", Value: ui.Status(downloadstation.StatusDisplay(t.Status), downloadstation.NormalizeStatus(t.Status))},
		{Label: "Type", Value: t.Type},
		{Label: "Destination", Value: valueOrDash(downloadstation.DestinationOf(t))},
		{Label: "Size", Value: cmdutil.FormatBytes(t.Size)},
		{Label: "Downloaded", Value: cmdutil.FormatBytes(downloadstation.DownloadedOf(t))},
		{Label: "Progress", Value: cmdutil.FormatPercent(downloadstation.DownloadedOf(t), t.Size)},
		{Label: "Uploaded", Value: cmdutil.FormatBytes(downloadstation.UploadedOf(t))},
		{Label: "Down Speed", Value: cmdutil.FormatSpeed(downloadstation.DownSpeedOf(t))},
		{Label: "ETA", Value: formatTaskETA(t)},
		{Label: "Up Speed", Value: cmdutil.FormatSpeed(downloadstation.UpSpeedOf(t))},
		{Label: "URI", Value: valueOrDash(downloadstation.URIOf(t))},
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
	ui := cmdutil.NewHumanUI(w)
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{
			t.ID,
			t.Title,
			ui.Status(downloadstation.StatusDisplay(t.Status), downloadstation.NormalizeStatus(t.Status)),
			t.Type,
			valueOrDash(downloadstation.DestinationOf(t)),
			cmdutil.FormatBytes(t.Size),
			cmdutil.FormatBytes(downloadstation.DownloadedOf(t)),
			cmdutil.FormatPercent(downloadstation.DownloadedOf(t), t.Size),
			cmdutil.FormatBytes(downloadstation.UploadedOf(t)),
			cmdutil.FormatSpeed(downloadstation.DownSpeedOf(t)),
			formatTaskETA(t),
			cmdutil.FormatSpeed(downloadstation.UpSpeedOf(t)),
		})
	}
	cmdutil.PrintTable(w, []string{"ID", "Title", "Status", "Type", "Destination", "Size", "Downloaded", "Progress", "Uploaded", "Down Speed", "ETA", "Up Speed"}, rows)
}

func formatTaskETA(t downloadstation.Task) string {
	etaSeconds := downloadstation.ETASecondsOf(t)
	if etaSeconds < 0 {
		return "-"
	}
	return cmdutil.FormatDurationWords(etaSeconds)
}
