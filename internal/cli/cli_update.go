package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
	"synocli/internal/output"
)

func newCLIUpdateCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   updateCommandName,
		Short: "Update synocli to the latest GitHub release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			current := strings.TrimSpace(versionValue())
			statePath, err := ac.updateStatePath()
			if err != nil {
				return ac.outputError(updateCommandName, "", start, apperr.Wrap("internal_error", "resolve update state path", 1, err))
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()

			client := buildUpdateClient(60 * time.Second)
			check, err := client.CheckForUpdate(ctx, current, statePath, true)
			if err != nil {
				return ac.outputError(updateCommandName, "", start, apperr.Wrap("update_check_failed", "check latest release", 1, err))
			}
			if !check.UpdateAvailable {
				data := map[string]any{
					"current_version": current,
					"latest_version":  check.LatestVersion,
					"updated":         false,
				}
				if ac.opts.JSON {
					env := output.NewEnvelope(true, updateCommandName, "", start)
					env.Data = data
					if err := output.WriteJSON(ac.out, env); err != nil {
						return apperr.Wrap("internal_error", "write json output", 1, err)
					}
					return nil
				}
				cmdutil.PrintKVBlock(ac.out, "CLI Update", []cmdutil.KVField{
					{Label: "Current", Value: current},
					{Label: "Latest", Value: check.LatestVersion},
					{Label: "Result", Value: "already up to date"},
				})
				return nil
			}

			rel, err := client.FetchLatestRelease(ctx)
			if err != nil {
				return ac.outputError(updateCommandName, "", start, apperr.Wrap("update_check_failed", "fetch latest release assets", 1, err))
			}
			exePath, err := os.Executable()
			if err != nil {
				return ac.outputError(updateCommandName, "", start, apperr.Wrap("internal_error", "resolve executable path", 1, err))
			}
			apply, err := client.ApplyUpdate(ctx, rel, current, exePath, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				return ac.outputError(updateCommandName, "", start, apperr.Wrap("update_apply_failed", "apply update", 1, err))
			}

			data := map[string]any{
				"current_version": apply.CurrentVersion,
				"latest_version":  apply.LatestVersion,
				"updated":         apply.Updated,
				"binary_path":     apply.BinaryPath,
			}
			if ac.opts.JSON {
				env := output.NewEnvelope(true, updateCommandName, "", start)
				env.Data = data
				if err := output.WriteJSON(ac.out, env); err != nil {
					return apperr.Wrap("internal_error", "write json output", 1, err)
				}
				return nil
			}
			result := "updated"
			if !apply.Updated {
				result = "already up to date"
			}
			cmdutil.PrintKVBlock(ac.out, "CLI Update", []cmdutil.KVField{
				{Label: "Current", Value: apply.CurrentVersion},
				{Label: "Latest", Value: apply.LatestVersion},
				{Label: "Binary", Value: apply.BinaryPath},
				{Label: "Result", Value: result},
			})
			if apply.Updated {
				_, _ = fmt.Fprintln(ac.out)
				_, _ = fmt.Fprintln(ac.out, "Restart running shell sessions if they cache command paths.")
			}
			return nil
		},
	}
}
