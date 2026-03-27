package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
	"synocli/internal/output"
	"synocli/internal/synology/filestation"
)

func newTaskStartCmd(ac *appContext, apiKey, cmdName, title, method string, paramsFn func(args []string) (map[string]string, error)) *cobra.Command {
	var async bool
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <path>", cmdName),
		Short: title,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withSession(cmd, joinCommand("fs", cmdName), func(ctx context.Context, s *session) (any, error) {
				pairs, err := paramsFn(args)
				if err != nil {
					return nil, err
				}
				params := makeValuesFromMap(pairs)
				var out map[string]any
				if err := s.fsClient.Call(ctx, apiKey, method, params, &out); err != nil {
					return nil, err
				}
				taskID := filestation.FirstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "task id missing", 1)
				}
				if !async {
					status, err := s.fsClient.WaitTask(ctx, apiKey, taskID, interval, maxWait)
					if err != nil {
						return nil, err
					}
					out["status"] = status
				}
				out["task_id"] = taskID
				if ac.opts.JSON {
					return out, nil
				}
				cmdutil.PrintKVBlock(ac.out, title, []cmdutil.KVField{{Label: "Task ID", Value: taskID}})
				return nil, nil
			})
		},
	}
	cmd.Flags().BoolVar(&async, "async", false, "Do not wait for completion")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	return cmd
}

func newTaskStatusCmd(ac *appContext, apiKey, parentName, title string) *cobra.Command {
	return &cobra.Command{
		Use:   "status <task-id>",
		Short: title,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", parentName, "status"), func(ctx context.Context, s *session) (any, error) {
				var out map[string]any
				if err := s.fsClient.Call(ctx, apiKey, "status", makeValues("taskid", args[0]), &out); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				cmdutil.PrintKVBlock(ac.out, title, []cmdutil.KVField{{Label: "Task ID", Value: args[0]}, {Label: "Finished", Value: filestation.ValueFromMap(out, "finished")}})
				return nil, nil
			})
		},
	}
}

func newTaskStopCmd(ac *appContext, apiKey, parentName, title string) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <task-id>",
		Short: title,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", parentName, "stop"), func(ctx context.Context, s *session) (any, error) {
				if err := s.fsClient.Call(ctx, apiKey, "stop", makeValues("taskid", args[0]), nil); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"task_id": args[0], "stopped": true}, nil
				}
				cmdutil.PrintKVBlock(ac.out, title, []cmdutil.KVField{{Label: "Task ID", Value: args[0]}})
				return nil, nil
			})
		},
	}
}

func newFSTasksCmd(ac *appContext) *cobra.Command {
	var offset, limit int
	var sortBy, sortDirection string
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "List File Station background tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if offset < 0 {
				return apperr.New("validation_error", "--offset must be >= 0", 1)
			}
			if limit < 0 {
				return apperr.New("validation_error", "--limit must be >= 0", 1)
			}
			if watch {
				if err := validatePositiveDuration("--interval", interval); err != nil {
					return err
				}
			}
			return ac.withSession(cmd, joinCommand("fs", "tasks"), func(ctx context.Context, s *session) (any, error) {
				fetch := func() (map[string]any, error) {
					params := makeValues("offset", fmt.Sprintf("%d", offset), "limit", fmt.Sprintf("%d", limit))
					if sortBy != "" {
						params.Set("sort_by", sortBy)
					}
					if sortDirection != "" {
						params.Set("sort_direction", sortDirection)
					}
					var out map[string]any
					if err := s.fsClient.Call(ctx, filestation.APIBackgroundTask, "list", params, &out); err != nil {
						return nil, err
					}
					return out, nil
				}
				if watch {
					ui := cmdutil.NewHumanUI(ac.out)
					return nil, cmdutil.PollLoop(ctx, interval, func() error {
						out, err := fetch()
						if err != nil {
							return err
						}
						if ac.opts.JSON {
							env := output.NewEnvelope(true, joinCommand("fs", "tasks"), s.endpoint, s.start)
							env.Meta.APIVersion = s.apiVersions
							env.Data = map[string]any{"event": "snapshot", "mode": "tasks", "snapshot": out}
							return output.WriteJSONLine(ac.out, env)
						}
						if ui.Tty {
							_, _ = fmt.Fprint(ac.out, cmdutil.AnsiClearScreen)
						}
						cmdutil.PrintKVBlock(ac.out, "Background Tasks", []cmdutil.KVField{{Label: "Timestamp", Value: time.Now().Format(time.RFC3339)}})
						printBackgroundTasks(ac.out, out)
						return nil
					})
				}
				out, err := fetch()
				if err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return out, nil
				}
				printBackgroundTasks(ac.out, out)
				return nil, nil
			})
		},
	}
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset")
	cmd.Flags().IntVar(&limit, "limit", 100, "Limit")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort by")
	cmd.Flags().StringVar(&sortDirection, "sort-direction", "", "Sort direction")
	cmd.Flags().BoolVar(&watch, "watch", false, "Continuous polling mode")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	return cmd
}

func newFSTasksClearCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "tasks-clear [<task-id>...]",
		Short: "Clear finished background tasks",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("fs", "tasks-clear"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues()
				if len(args) > 0 {
					j, err := filestation.EncodeJSON(args)
					if err != nil {
						return nil, err
					}
					params.Set("taskid", j)
				}
				if err := s.fsClient.Call(ctx, filestation.APIBackgroundTask, "clear_finished", params, nil); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"cleared": true, "task_ids": args}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Tasks Clear", []cmdutil.KVField{{Label: "Cleared", Value: "true"}})
				return nil, nil
			})
		},
	}
}

func printBackgroundTasks(w io.Writer, out map[string]any) {
	tasks := filestation.MapSliceAny(out["tasks"])
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{filestation.ValueFromMap(t, "taskid"), filestation.ValueFromMap(t, "api"), filestation.ValueFromMap(t, "status"), filestation.ValueFromMap(t, "progress")})
	}
	cmdutil.PrintKVBlock(w, "Background Tasks", []cmdutil.KVField{{Label: "Count", Value: fmt.Sprintf("%d", len(tasks))}})
	cmdutil.PrintTable(w, []string{"Task ID", "API", "Status", "Progress"}, rows)
}
