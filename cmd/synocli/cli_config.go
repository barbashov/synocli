package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/config"
	"synocli/internal/output"
)

func newCLIConfigCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli-config",
		Short: "Manage synocli local config",
		Long: `Manage the synocli config file (default: ~/.synocli/config, chmod 600).

Supported directives:

  endpoint      = https://host:5001   DSM base URL (required)
  user          = admin               Synology username
  password      = secret              Synology password
  insecure_tls  = false               Skip TLS certificate verification
  timeout       = 30s                 Per-request timeout (Go duration)
  reuse_session = false               Cache session SID between calls`,
	}
	cmd.AddCommand(newCLIConfigInitCmd(ac), newCLIConfigShowCmd(ac))
	return cmd
}

func newCLIConfigInitCmd(ac *appContext) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			configPath, err := ac.configPath()
			if err != nil {
				return ac.outputError(joinCommand("cli-config", "init"), "", start, apperr.Wrap("validation_error", "invalid config path", 1, err))
			}
			if !force {
				if _, err := os.Stat(configPath); err == nil {
					return ac.outputError(joinCommand("cli-config", "init"), "", start, apperr.New("validation_error", "config file already exists; use --force to overwrite", 1))
				} else if !os.IsNotExist(err) {
					return ac.outputError(joinCommand("cli-config", "init"), "", start, apperr.Wrap("internal_error", "read config file", 1, err))
				}
			}
			if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
				return ac.outputError(joinCommand("cli-config", "init"), "", start, apperr.Wrap("internal_error", "create config directory", 1, err))
			}
			content := buildConfigContent(ac.opts)
			if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
				return ac.outputError(joinCommand("cli-config", "init"), "", start, apperr.Wrap("internal_error", "write config file", 1, err))
			}
			data := map[string]any{"path": configPath, "created": true}
			if ac.opts.JSON {
				env := output.NewEnvelope(true, joinCommand("cli-config", "init"), "", start)
				env.Data = data
				if err := output.WriteJSON(ac.out, env); err != nil {
					return apperr.Wrap("internal_error", "write json output", 1, err)
				}
				return nil
			}
			printKVBlock(ac.out, "Config Initialized", []kvField{
				{Label: "Path", Value: configPath},
				{Label: "Mode", Value: "0600"},
			})
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing config file")
	return cmd
}

func newCLIConfigShowCmd(ac *appContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show config values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			configPath, err := ac.configPath()
			if err != nil {
				return ac.outputError(joinCommand("cli-config", "show"), "", start, apperr.Wrap("validation_error", "invalid config path", 1, err))
			}
			cfg, err := config.LoadConfigFile(configPath, true)
			if err != nil {
				return ac.outputError(joinCommand("cli-config", "show"), "", start, apperr.Wrap("validation_error", "invalid config file", 1, err))
			}
			timeout := "-"
			if cfg.Timeout > 0 {
				timeout = cfg.Timeout.String()
			}
			data := map[string]any{
				"path":         configPath,
				"endpoint":     cfg.Endpoint,
				"user":         cfg.User,
				"password":     redactPassword(cfg.Password),
				"insecure_tls": cfg.InsecureTLS,
				"timeout":      timeout,
			}
			if ac.opts.JSON {
				env := output.NewEnvelope(true, joinCommand("cli-config", "show"), "", start)
				env.Data = data
				if err := output.WriteJSON(ac.out, env); err != nil {
					return apperr.Wrap("internal_error", "write json output", 1, err)
				}
				return nil
			}
			printKVBlock(ac.out, "Config", []kvField{
				{Label: "Path", Value: configPath},
				{Label: "Endpoint", Value: valueOrDash(cfg.Endpoint)},
				{Label: "User", Value: valueOrDash(cfg.User)},
				{Label: "Password", Value: redactPassword(cfg.Password)},
				{Label: "Insecure TLS", Value: fmt.Sprintf("%t", cfg.InsecureTLS)},
				{Label: "Timeout", Value: timeout},
			})
			return nil
		},
	}
	return cmd
}

func redactPassword(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return "<redacted>"
}

func buildConfigContent(cfg config.GlobalOptions) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "endpoint=%s\n", cfg.Endpoint)
	_, _ = fmt.Fprintf(&b, "user=%s\n", cfg.User)
	_, _ = fmt.Fprintf(&b, "password=%s\n", cfg.Password)
	_, _ = fmt.Fprintf(&b, "insecure_tls=%t\n", cfg.InsecureTLS)
	if cfg.Timeout > 0 {
		_, _ = fmt.Fprintf(&b, "timeout=%s\n", cfg.Timeout)
	}
	return b.String()
}

func (a *appContext) configPath() (string, error) {
	if strings.TrimSpace(a.opts.ConfigPath) != "" {
		return a.opts.ConfigPath, nil
	}
	return config.DefaultConfigPath()
}
