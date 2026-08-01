package cli

import (
	"errors"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/cmdutil"
	"synocli/internal/config"
)

// Execute is the package entry point called by cmd/synocli/main.go.
func Execute(stdin io.Reader, stdout, stderr io.Writer) error {
	root, ac := newRootCmd(stdin, stdout, stderr)
	return runRoot(root, ac, stderr)
}

// runRoot executes the root command and routes errors that never reached
// withSession (flag/arg validation, cobra usage) through the same output
// contract: a JSON envelope in --json mode, plain stderr otherwise.
func runRoot(root *cobra.Command, ac *appContext, stderr io.Writer) error {
	start := time.Now()
	cmd, err := root.ExecuteC()
	if err == nil {
		return nil
	}
	var handled *jsonOutputHandledError
	if errors.As(err, &handled) {
		return err
	}
	if ac.opts.JSON {
		name := strings.TrimSpace(strings.TrimPrefix(cmd.CommandPath(), root.Name()))
		return ac.outputError(name, ac.opts.Endpoint, start, toAppError(err))
	}
	cmdutil.PrintError(stderr, err)
	return err
}

func newRootCmd(stdin io.Reader, stdout, stderr io.Writer) (*cobra.Command, *appContext) {
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
			if cmd.Flags().Lookup("password") != nil && cmd.Flags().Lookup("password").Changed && cmdutil.IsTerminal(stderr) {
				_, _ = io.WriteString(stderr, "warning: --password exposes the password to other local users via the process list; prefer --password-stdin or --credentials-file\n")
			}
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

	cmd.AddCommand(newAuthCmd(ac), newDSCmd(ac), newFSCmd(ac), newInfoCmd(ac), newCLIConfigCmd(ac), newCLIUpdateCmd(ac), newVersionCmd(ac))
	return cmd, ac
}
