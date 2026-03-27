package cli

import (
	"errors"
	"io"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/cmdutil"
	"synocli/internal/config"
)

// Execute is the package entry point called by cmd/synocli/main.go.
func Execute(stdin io.Reader, stdout, stderr io.Writer) error {
	root := newRootCmd(stdin, stdout, stderr)
	if err := root.Execute(); err != nil {
		var handled *jsonOutputHandledError
		if !errors.As(err, &handled) {
			cmdutil.PrintError(stderr, err)
		}
		return err
	}
	return nil
}

func newRootCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	ac := &appContext{stdin: stdin, out: stdout, err: stderr}
	defaultConfigPath, _ := config.DefaultConfigPath()
	ac.opts.ConfigPath = defaultConfigPath
	cmd := &cobra.Command{
		Use:           "synocli",
		Short:         "Synology DSM CLI",
		Version:       versionValue(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ac.maybeNotifyUpdate(cmd)
			return nil
		},
	}
	cmd.SetVersionTemplate("{{.Version}}\n")
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	f := cmd.PersistentFlags()
	f.StringVar(&ac.opts.Endpoint, "endpoint", "", "Synology DSM endpoint (https://host:5001)")
	f.StringVar(&ac.opts.ConfigPath, "config", ac.opts.ConfigPath, "Path to per-user synocli config file")
	f.StringVar(&ac.opts.User, "user", "", "Synology username")
	f.StringVar(&ac.opts.Password, "password", "", "Synology password")
	f.BoolVar(&ac.opts.PasswordStdin, "password-stdin", false, "Read password from stdin")
	f.StringVar(&ac.opts.CredentialsFile, "credentials-file", "", "Path to credentials file (user=..., password=...)")
	f.BoolVar(&ac.opts.InsecureTLS, "insecure-tls", false, "Allow insecure TLS (self-signed certs)")
	f.DurationVar(&ac.opts.Timeout, "timeout", 30*time.Second, "Request timeout")
	f.BoolVar(&ac.opts.JSON, "json", false, "JSON output")
	f.BoolVar(&ac.opts.NoUpdateCheck, "no-update-check", false, "Skip background update check for this invocation")
	f.BoolVar(&ac.opts.Debug, "debug", false, "Debug request flow")

	cmd.AddCommand(newAuthCmd(ac), newDSCmd(ac), newFSCmd(ac), newCLIConfigCmd(ac), newCLIUpdateCmd(ac), newVersionCmd(ac))
	return cmd
}
