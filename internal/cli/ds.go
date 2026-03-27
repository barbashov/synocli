package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

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
			cmdutil.FormatSpeed(downloadstation.UpSpeedOf(t)),
		})
	}
	cmdutil.PrintTable(w, []string{"ID", "Title", "Status", "Type", "Destination", "Size", "Downloaded", "Progress", "Uploaded", "Down Speed", "Up Speed"}, rows)
}
