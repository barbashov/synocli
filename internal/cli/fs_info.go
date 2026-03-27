package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
	"synocli/internal/synology/filestation"
)

func newFSDirSizeCmd(ac *appContext) *cobra.Command {
	cmd := newTaskStartCmd(ac, filestation.APIDirSize, "dir-size", "Calculate directory size", "start", func(args []string) (map[string]string, error) {
		j, err := filestation.EncodeJSON(args)
		if err != nil {
			return nil, err
		}
		return map[string]string{"path": j}, nil
	})
	cmd.AddCommand(
		newTaskStatusCmd(ac, filestation.APIDirSize, "dir-size", "Check dir size status"),
		newTaskStopCmd(ac, filestation.APIDirSize, "dir-size", "Stop dir size calculation"),
	)
	return cmd
}

func newFSMD5Cmd(ac *appContext) *cobra.Command {
	cmd := newTaskStartCmd(ac, filestation.APIMD5, "md5", "Calculate MD5 checksum", "start", func(args []string) (map[string]string, error) {
		return map[string]string{"file_path": args[0]}, nil
	})
	cmd.AddCommand(
		newTaskStatusCmd(ac, filestation.APIMD5, "md5", "Check MD5 status"),
		newTaskStopCmd(ac, filestation.APIMD5, "md5", "Stop MD5 calculation"),
	)
	return cmd
}

func newFSSearchCmd(ac *appContext) *cobra.Command {
	var pattern string
	var recursive bool
	var filetype string
	var async bool
	var interval time.Duration
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   "search <folder-path>",
		Short: "Search files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" {
				return apperr.New("validation_error", "--pattern is required", 1)
			}
			if err := validatePositiveDuration("--interval", interval); err != nil {
				return err
			}
			return ac.withSession(cmd, joinCommand("fs", "search"), func(ctx context.Context, s *session) (any, error) {
				params := makeValues("folder_path", args[0], "pattern", pattern, "recursive", fmt.Sprintf("%t", recursive))
				if filetype != "" {
					params.Set("filetype", filetype)
				}
				var out map[string]any
				if err := s.fsClient.Call(ctx, filestation.APISearch, "start", params, &out); err != nil {
					return nil, err
				}
				taskID := filestation.FirstTaskID(out)
				if taskID == "" {
					return nil, apperr.New("internal_error", "search task id missing", 1)
				}
				res := map[string]any{"task_id": taskID}
				if !async {
					snapshot, err := s.fsClient.WaitSearch(ctx, taskID, interval, maxWait)
					if err != nil {
						return nil, err
					}
					res["result"] = snapshot
				}
				if ac.opts.JSON {
					return res, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Search", []cmdutil.KVField{{Label: "Task ID", Value: taskID}, {Label: "Pattern", Value: pattern}})
				return nil, nil
			})
		},
	}
	cmd.Flags().StringVar(&pattern, "pattern", "", "Pattern")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "Recursive search")
	cmd.Flags().StringVar(&filetype, "file-type", "", "file/dir/all")
	cmd.Flags().BoolVar(&async, "async", false, "Do not wait for completion")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&maxWait, "max-wait", 0, "Maximum wait duration (0 means unlimited)")
	cmd.AddCommand(
		&cobra.Command{
			Use:   "results <task-id>",
			Short: "Get search results",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return ac.withSession(cmd, joinCommand("fs", "search", "results"), func(ctx context.Context, s *session) (any, error) {
					params := makeValues("taskid", args[0], "offset", "0", "limit", "0")
					var out map[string]any
					if err := s.fsClient.Call(ctx, filestation.APISearch, "list", params, &out); err != nil {
						return nil, err
					}
					if ac.opts.JSON {
						return out, nil
					}
					files := filestation.MapSliceAny(out["files"])
					rows := make([][]string, 0, len(files))
					for _, f := range files {
						rows = append(rows, []string{filestation.ValueFromMap(f, "path"), filestation.ValueFromMap(f, "name")})
					}
					cmdutil.PrintTable(ac.out, []string{"Path", "Name"}, rows)
					return nil, nil
				})
			},
		},
		&cobra.Command{
			Use:   "stop <task-id>",
			Short: "Stop search task",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return ac.withSession(cmd, joinCommand("fs", "search", "stop"), func(ctx context.Context, s *session) (any, error) {
					if err := s.fsClient.Call(ctx, filestation.APISearch, "stop", makeValues("taskid", args[0]), nil); err != nil {
						return nil, err
					}
					if ac.opts.JSON {
						return map[string]any{"task_id": args[0], "stopped": true}, nil
					}
					cmdutil.PrintKVBlock(ac.out, "Search Stop", []cmdutil.KVField{{Label: "Task ID", Value: args[0]}})
					return nil, nil
				})
			},
		},
		&cobra.Command{
			Use:   "clear <task-id> [<task-id>...]",
			Short: "Clear search tasks",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return ac.withSession(cmd, joinCommand("fs", "search", "clear"), func(ctx context.Context, s *session) (any, error) {
					for _, taskID := range args {
						if err := s.fsClient.Call(ctx, filestation.APISearch, "clean", makeValues("taskid", taskID), nil); err != nil {
							return nil, err
						}
					}
					if ac.opts.JSON {
						return map[string]any{"cleared": true, "task_ids": args}, nil
					}
					cmdutil.PrintKVBlock(ac.out, "Search", []cmdutil.KVField{
						{Label: "Cleared", Value: "true"},
						{Label: "Task IDs", Value: strings.Join(args, ", ")},
					})
					return nil, nil
				})
			},
		},
	)
	return cmd
}
