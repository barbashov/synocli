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
	if !cmd.Flags().Lookup("password").Changed && strings.TrimSpace(fileCfg.Password) != "" {
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
		out.User = ""
		out.Password = ""
	}
	if out.Password != "" && out.PasswordStdin {
		return config.GlobalOptions{}, errors.New("use only one of --password or --password-stdin")
	}
	return out, nil
}
