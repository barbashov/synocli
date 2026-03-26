package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"synocli/internal/synology/apiinfo"
)

func newAuthCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authentication and connectivity commands"}
	cmd.AddCommand(newAuthPingCmd(ac), newAuthWhoamiCmd(ac), newAuthAPIInfoCmd(ac))
	return cmd
}

func newAuthPingCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Validate connectivity and auth",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withAuthSession(cmd, joinCommand("auth", "ping"), func(ctx context.Context, s *session) (any, error) {
				if ac.opts.JSON {
					return map[string]any{"status": "ok", "user": ac.opts.User}, nil
				}
				printKVBlock(ac.out, "Authentication", []kvField{
					{Label: "Result", Value: "ok"},
					{Label: "User", Value: ac.opts.User},
				})
				return nil, nil
			})
		},
	}
}

func newAuthWhoamiCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show authenticated user context",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withAuthSession(cmd, joinCommand("auth", "whoami"), func(ctx context.Context, s *session) (any, error) {
				data := map[string]any{"user": ac.opts.User, "authenticated": true}
				if ac.opts.JSON {
					return data, nil
				}
				printKVBlock(ac.out, "Identity", []kvField{
					{Label: "User", Value: ac.opts.User},
					{Label: "Authenticated", Value: "true"},
				})
				return nil, nil
			})
		},
	}
}

func newAuthAPIInfoCmd(ac *appContext) *cobra.Command {
	var prefix string
	cmd := &cobra.Command{
		Use:   "api-info",
		Short: "Show discovered DSM APIs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withAuthSession(cmd, joinCommand("auth", "api-info"), func(ctx context.Context, s *session) (any, error) {
				entries, err := apiinfo.Discover(ctx, s.endpoint, s.authClient.HTTP)
				if err != nil {
					return nil, err
				}
				filtered := make(map[string]apiinfo.Entry)
				for k, v := range entries {
					if prefix == "" || strings.HasPrefix(k, prefix) {
						filtered[k] = v
					}
				}
				if ac.opts.JSON {
					return map[string]any{"apis": filtered}, nil
				}
				keys := make([]string, 0, len(filtered))
				for k := range filtered {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				printKVBlock(ac.out, "DSM API Info", []kvField{
					{Label: "Matched", Value: fmt.Sprintf("%d", len(keys))},
					{Label: "Prefix", Value: valueOrDash(prefix)},
				})
				if len(keys) == 0 {
					ui := newHumanUI(ac.out)
					_, _ = fmt.Fprintln(ac.out, ui.muted("No APIs matched the current filter."))
					return nil, nil
				}
				rows := make([][]string, 0, len(keys))
				for _, k := range keys {
					e := filtered[k]
					rows = append(rows, []string{k, e.Path, fmt.Sprintf("%d", e.MinVersion), fmt.Sprintf("%d", e.MaxVersion)})
				}
				printTable(ac.out, []string{"API", "Path", "Min", "Max"}, rows)
				return nil, nil
			})
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "Filter APIs by prefix")
	return cmd
}
