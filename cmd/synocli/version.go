package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/output"
)

var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

func versionValue() string {
	v := strings.TrimSpace(buildVersion)
	if v == "" {
		return "dev"
	}
	return v
}

func versionData() map[string]string {
	commit := strings.TrimSpace(buildCommit)
	if commit == "" {
		commit = "none"
	}
	buildDateValue := strings.TrimSpace(buildDate)
	if buildDateValue == "" {
		buildDateValue = "unknown"
	}
	return map[string]string{
		"version":    versionValue(),
		"commit":     commit,
		"build_date": buildDateValue,
	}
}

func newVersionCmd(ac *appContext) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show CLI version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			data := versionData()
			if ac.opts.JSON {
				env := output.NewEnvelope(true, joinCommand("version"), "", start)
				env.Data = data
				return output.WriteJSON(ac.out, env)
			}
			_, err := fmt.Fprintf(ac.out, "synocli %s (commit %s, built %s)\n", data["version"], data["commit"], data["build_date"])
			return err
		},
	}
}
