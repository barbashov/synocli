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
		Use:   "ping <endpoint>",
		Short: "Validate connectivity and auth",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withAuthSession(cmd, args[0], joinCommand("auth", "ping"), func(ctx context.Context, s *session) (any, error) {
				if ac.opts.JSON {
					return map[string]any{"status": "ok", "user": ac.opts.User}, nil
				}
				fmt.Fprintf(ac.out, "ok: authenticated as %s\n", ac.opts.User)
				return nil, nil
			})
		},
	}
}

func newAuthWhoamiCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami <endpoint>",
		Short: "Show authenticated user context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withAuthSession(cmd, args[0], joinCommand("auth", "whoami"), func(ctx context.Context, s *session) (any, error) {
				data := map[string]any{"user": ac.opts.User, "authenticated": true}
				if ac.opts.JSON {
					return data, nil
				}
				fmt.Fprintf(ac.out, "user: %s\nauthenticated: true\n", ac.opts.User)
				return nil, nil
			})
		},
	}
}

func newAuthAPIInfoCmd(ac *appContext) *cobra.Command {
	var prefix string
	cmd := &cobra.Command{
		Use:   "api-info <endpoint>",
		Short: "Show discovered DSM APIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withAuthSession(cmd, args[0], joinCommand("auth", "api-info"), func(ctx context.Context, s *session) (any, error) {
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
				fmt.Fprintln(ac.out, "api\tpath\tmin\tmax")
				for _, k := range keys {
					e := filtered[k]
					fmt.Fprintf(ac.out, "%s\t%s\t%d\t%d\n", k, e.Path, e.MinVersion, e.MaxVersion)
				}
				return nil, nil
			})
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "Filter APIs by prefix")
	return cmd
}
