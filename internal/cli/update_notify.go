package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/config"
	"synocli/internal/update"
)

const updateCommandName = "cli-update"

var buildUpdateClient = func(timeout time.Duration) *update.Client {
	return update.NewClient(&http.Client{Timeout: timeout})
}

func (a *appContext) maybeNotifyUpdate(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if a.opts.JSON || a.opts.NoUpdateCheck {
		return
	}
	if cmd.Name() == updateCommandName {
		return
	}
	current := strings.TrimSpace(versionValue())
	if current == "" || current == "dev" {
		return
	}
	statePath, err := a.updateStatePath()
	if err != nil {
		if a.opts.Debug {
			_, _ = fmt.Fprintf(a.err, "[debug] update state path: %v\n", err)
		}
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	client := buildUpdateClient(3 * time.Second)
	res, err := client.CheckForUpdate(ctx, current, statePath, false)
	if err != nil {
		if a.opts.Debug {
			_, _ = fmt.Fprintf(a.err, "[debug] update check failed: %v\n", err)
		}
		return
	}
	if !res.UpdateAvailable {
		return
	}
	_, _ = fmt.Fprintf(a.err, "A new synocli version %s is available (current %s). Run: synocli cli-update\n", res.LatestVersion, current)
}

func (a *appContext) updateStatePath() (string, error) {
	configPath := strings.TrimSpace(a.opts.ConfigPath)
	if configPath == "" {
		var err error
		configPath, err = config.DefaultConfigPath()
		if err != nil {
			return "", err
		}
	}
	return update.StatePathFromConfig(configPath), nil
}
