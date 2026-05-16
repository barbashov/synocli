package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
	"synocli/internal/synology/downloadstation"
)

func newDSMaxTasksCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "max-tasks", Short: "Manage Download Station maximum active download tasks"}
	cmd.AddCommand(
		newDSMaxTasksGetCmd(ac),
		newDSMaxTasksSetCmd(ac),
	)
	return cmd
}

func newDSMaxTasksGetCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show current maximum active download tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("ds", "max-tasks", "get"), func(ctx context.Context, s *session) (any, error) {
				cfg, err := s.dsClient.GetSchedulerConfig(ctx)
				if err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{
						"max_tasks":       cfg.MaxTasks,
						"max_tasks_limit": cfg.MaxTasksLimit,
					}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Max Active Download Tasks", []cmdutil.KVField{
					{Label: "Current", Value: strconv.Itoa(cfg.MaxTasks)},
					{Label: "Limit", Value: strconv.Itoa(cfg.MaxTasksLimit)},
				})
				return nil, nil
			})
		},
	}
}

func newDSMaxTasksSetCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "set <n>",
		Short: "Set maximum active download tasks",
		Long:  "Set the maximum number of simultaneously active download tasks.\n\nThe value must be a positive integer within DSM's upper bound (see `ds max-tasks get` -> Limit). Values above the limit are rejected client-side because DSM accepts them silently but the Download Station UI considers them invalid.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return apperr.New("validation_error", fmt.Sprintf("max-tasks must be an integer, got %q", args[0]), 1)
			}
			if n < 1 {
				return apperr.New("validation_error", "max-tasks must be >= 1", 1)
			}
			update := downloadstation.SchedulerConfigUpdate{MaxTasks: &n}
			return ac.withSession(cmd, joinCommand("ds", "max-tasks", "set"), func(ctx context.Context, s *session) (any, error) {
				// DSM accepts any integer here silently, so we have to fetch
				// max_tasks_limit and enforce the upper bound ourselves to
				// match the Download Station UI's validation.
				cfg, err := s.dsClient.GetSchedulerConfig(ctx)
				if err != nil {
					return nil, err
				}
				if n > cfg.MaxTasksLimit {
					return nil, apperr.New("validation_error", fmt.Sprintf("max-tasks must be <= %d (DSM limit)", cfg.MaxTasksLimit), 1)
				}
				if err := s.dsClient.SetSchedulerConfig(ctx, update); err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{"max_tasks": n}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Max Active Download Tasks Updated", []cmdutil.KVField{
					{Label: "Current", Value: strconv.Itoa(n)},
				})
				return nil, nil
			})
		},
	}
}
