package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"synocli/internal/config"
)

func (a *appContext) resolveRuntimeOptions(cmd *cobra.Command) (config.GlobalOptions, error) {
	out := a.opts
	configPath := strings.TrimSpace(out.ConfigPath)
	if configPath == "" {
		var err error
		configPath, err = config.DefaultConfigPath()
		if err != nil {
			return config.GlobalOptions{}, err
		}
	}
	out.ConfigPath = configPath
	fileCfg, err := config.LoadConfigFile(configPath, cmd.Flags().Lookup("config").Changed)
	if err != nil {
		return config.GlobalOptions{}, err
	}

	if !cmd.Flags().Lookup("endpoint").Changed && strings.TrimSpace(fileCfg.Endpoint) != "" {
		out.Endpoint = fileCfg.Endpoint
	}
	if !cmd.Flags().Lookup("user").Changed && strings.TrimSpace(fileCfg.User) != "" {
		out.User = fileCfg.User
	}
	// An explicit --password-stdin outranks the config-file password; merging
	// the file value here would trip the mutual-exclusion check below.
	if !cmd.Flags().Lookup("password").Changed && !out.PasswordStdin && strings.TrimSpace(fileCfg.Password) != "" {
		out.Password = fileCfg.Password
	}
	if !cmd.Flags().Lookup("insecure-tls").Changed {
		out.InsecureTLS = fileCfg.InsecureTLS
	}
	if !cmd.Flags().Lookup("timeout").Changed && fileCfg.Timeout > 0 {
		out.Timeout = fileCfg.Timeout
	}
	out.ReuseSession = fileCfg.ReuseSession

	if out.CredentialsFile != "" {
		if cmd.Flags().Lookup("user").Changed || cmd.Flags().Lookup("password").Changed || out.PasswordStdin {
			return config.GlobalOptions{}, errors.New("use --credentials-file without --user, --password, or --password-stdin")
		}
		// The credentials file is authoritative; load it now (rather than only at
		// login time) so identity output (whoami/ping) is correct even when a
		// cached session skips the login that would otherwise populate User.
		out.User = ""
		out.Password = ""
		if err := out.LoadCredentialsFile(); err != nil {
			return config.GlobalOptions{}, err
		}
	}
	if out.Password != "" && out.PasswordStdin {
		return config.GlobalOptions{}, errors.New("use only one of --password or --password-stdin")
	}
	return out, nil
}
