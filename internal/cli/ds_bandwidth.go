package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/cmdutil"
	"synocli/internal/synology/downloadstation"
)

func newDSBandwidthCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{Use: "bandwidth", Short: "Manage Download Station bandwidth limits"}
	cmd.AddCommand(
		newDSBandwidthGetCmd(ac),
		newDSBandwidthSetCmd(ac),
	)
	return cmd
}

func newDSBandwidthGetCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show current BT bandwidth limits",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ac.withSession(cmd, joinCommand("ds", "bandwidth", "get"), func(ctx context.Context, s *session) (any, error) {
				cfg, err := s.dsClient.GetServerConfig(ctx)
				if err != nil {
					return nil, err
				}
				if ac.opts.JSON {
					return map[string]any{
						"bt_max_download": cfg.BTMaxDownload,
						"bt_max_upload":   cfg.BTMaxUpload,
					}, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Bandwidth", []cmdutil.KVField{
					{Label: "BT Max Download", Value: formatSpeedLimit(cfg.BTMaxDownload)},
					{Label: "BT Max Upload", Value: formatSpeedLimit(cfg.BTMaxUpload)},
				})
				return nil, nil
			})
		},
	}
}

func newDSBandwidthSetCmd(ac *appContext) *cobra.Command {
	var btDown, btUp int
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update BT bandwidth limits (KB/s; 0 = unlimited)",
		Long:  "Update BT bandwidth limits in KB/s. Use 0 to remove a cap (unlimited).\n\nAt least one of --bt-max-download or --bt-max-upload is required.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			downChanged := cmd.Flags().Changed("bt-max-download")
			upChanged := cmd.Flags().Changed("bt-max-upload")
			if !downChanged && !upChanged {
				return apperr.New("validation_error", "at least one of --bt-max-download or --bt-max-upload is required", 1)
			}
			if downChanged && btDown < 0 {
				return apperr.New("validation_error", "--bt-max-download must be >= 0 (0 = unlimited)", 1)
			}
			if upChanged && btUp < 0 {
				return apperr.New("validation_error", "--bt-max-upload must be >= 0 (0 = unlimited)", 1)
			}
			update := downloadstation.ServerConfigUpdate{}
			if downChanged {
				update.BTMaxDownload = &btDown
			}
			if upChanged {
				update.BTMaxUpload = &btUp
			}
			return ac.withSession(cmd, joinCommand("ds", "bandwidth", "set"), func(ctx context.Context, s *session) (any, error) {
				if err := s.dsClient.SetServerConfig(ctx, update); err != nil {
					return nil, err
				}
				data := map[string]any{}
				fields := []cmdutil.KVField{}
				if downChanged {
					data["bt_max_download"] = btDown
					fields = append(fields, cmdutil.KVField{Label: "BT Max Download", Value: formatSpeedLimit(btDown)})
				} else {
					fields = append(fields, cmdutil.KVField{Label: "BT Max Download", Value: "-"})
				}
				if upChanged {
					data["bt_max_upload"] = btUp
					fields = append(fields, cmdutil.KVField{Label: "BT Max Upload", Value: formatSpeedLimit(btUp)})
				} else {
					fields = append(fields, cmdutil.KVField{Label: "BT Max Upload", Value: "-"})
				}
				if ac.opts.JSON {
					return data, nil
				}
				cmdutil.PrintKVBlock(ac.out, "Bandwidth Updated", fields)
				return nil, nil
			})
		},
	}
	cmd.Flags().IntVar(&btDown, "bt-max-download", 0, "BT max download in KB/s (0 = unlimited)")
	cmd.Flags().IntVar(&btUp, "bt-max-upload", 0, "BT max upload in KB/s (0 = unlimited)")
	return cmd
}

func formatSpeedLimit(kbps int) string {
	if kbps == 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d KB/s", kbps)
}
