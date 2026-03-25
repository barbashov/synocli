package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"synocli/internal/apperr"
	"synocli/internal/config"
	"synocli/internal/httpclient"
	"synocli/internal/output"
	"synocli/internal/synology/apiinfo"
	"synocli/internal/synology/auth"
	"synocli/internal/synology/downloadstation"
)

type appContext struct {
	opts  config.GlobalOptions
	stdin io.Reader
	out   io.Writer
	err   io.Writer
}

type session struct {
	endpoint    string
	start       time.Time
	authClient  *auth.Client
	dsClient    *downloadstation.Client
	sid         string
	apiVersions map[string]int
}

var (
	taskAPIRe     = regexp.MustCompile(`^SYNO\.DownloadStation(\d*)\.Task$`)
	taskListAPIRe = regexp.MustCompile(`^SYNO\.DownloadStation(\d*)\.Task\.List$`)
)

func newRootCmd(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	ac := &appContext{stdin: stdin, out: stdout, err: stderr}
	cmd := &cobra.Command{
		Use:           "synocli",
		Short:         "Synology DSM CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	f := cmd.PersistentFlags()
	f.StringVar(&ac.opts.User, "user", "", "Synology username")
	f.StringVar(&ac.opts.Password, "password", "", "Synology password")
	f.BoolVar(&ac.opts.PasswordStdin, "password-stdin", false, "Read password from stdin")
	f.StringVar(&ac.opts.CredentialsFile, "credentials-file", "", "Path to credentials file (user=..., password=...)")
	f.BoolVar(&ac.opts.InsecureTLS, "insecure-tls", false, "Allow insecure TLS (self-signed certs)")
	f.DurationVar(&ac.opts.Timeout, "timeout", 30*time.Second, "Request timeout")
	f.BoolVar(&ac.opts.JSON, "json", false, "JSON output")
	f.BoolVar(&ac.opts.Debug, "debug", false, "Debug request flow")

	cmd.AddCommand(newAuthCmd(ac), newDSCmd(ac))
	return cmd
}

func (a *appContext) withSession(cmd *cobra.Command, endpointRaw, commandName string, fn func(context.Context, *session) (any, error)) error {
	start := time.Now()
	u, err := config.ValidateEndpoint(endpointRaw)
	if err != nil {
		return a.outputError(commandName, endpointRaw, start, apperr.Wrap("validation_error", "invalid endpoint", 1, err))
	}
	if err := a.validateAuthOptions(); err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("validation_error", "invalid auth options", 1, err))
	}
	if err := a.opts.ResolvePassword(a.stdin); err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("validation_error", "invalid auth options", 1, err))
	}

	hc, err := httpclient.New(httpclient.Options{InsecureTLS: a.opts.InsecureTLS, Timeout: a.opts.Timeout, Debug: a.opts.Debug, DebugOut: a.err})
	if err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("internal_error", "initialize http client", 1, err))
	}
	ctx := cmd.Context()

	entries, err := apiinfo.Discover(ctx, u.String(), hc)
	if err != nil {
		entries = map[string]apiinfo.Entry{}
	}
	authPath, authVersion := apiinfo.Select(entries, "SYNO.API.Auth", "/webapi/auth.cgi", 6)
	dsAPIName, dsPath, dsVersion, dsListAPIName, dsListPath, dsListVersion := selectDownloadStationAPIs(entries)
	authVersion = clampVersion(authVersion, 6)
	dsVersion = clampVersion(dsVersion, 3)
	if dsListVersion > 0 {
		dsListVersion = clampVersion(dsListVersion, 3)
	}
	if a.opts.Debug {
		fmt.Fprintf(a.err, "[debug] selected task api=%s path=%s version=%d\n", dsAPIName, dsPath, dsVersion)
		if dsListAPIName != "" {
			fmt.Fprintf(a.err, "[debug] selected task-list api=%s path=%s version=%d\n", dsListAPIName, dsListPath, dsListVersion)
		}
	}
	authClient := &auth.Client{Endpoint: u.String(), Path: authPath, Version: authVersion, HTTP: hc}
	sid, err := authClient.Login(ctx, a.opts.User, a.opts.Password)
	if err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("auth_failed", "authentication failed", 2, err))
	}
	defer func() {
		_ = authClient.Logout(context.Background(), sid)
	}()
	s := &session{
		endpoint:   u.String(),
		start:      start,
		authClient: authClient,
		dsClient: &downloadstation.Client{
			Endpoint:    u.String(),
			Path:        dsPath,
			Version:     dsVersion,
			APIName:     dsAPIName,
			ListPath:    dsListPath,
			ListVersion: dsListVersion,
			ListAPIName: dsListAPIName,
			HTTP:        hc,
		},
		sid: sid,
		apiVersions: map[string]int{
			"auth": authVersion,
			"task": dsVersion,
		},
	}
	data, err := fn(ctx, s)
	if err != nil {
		return a.outputError(commandName, u.String(), start, toAppError(err))
	}
	env := output.NewEnvelope(true, commandName, u.String(), start)
	env.Data = data
	env.Meta.APIVersion = s.apiVersions
	if a.opts.JSON {
		if err := output.WriteJSON(a.out, env); err != nil {
			return apperr.Wrap("internal_error", "write json output", 1, err)
		}
		return nil
	}
	return nil
}

func toAppError(err error) error {
	var dsErr *downloadstation.APIError
	if errors.As(err, &dsErr) {
		code := "synology_error"
		exit := 1
		if dsErr.Code == 401 || dsErr.Code == 402 {
			exit = 3
		}
		return &apperr.Error{
			Code:     code,
			Message:  downloadstation.ErrorMessage(dsErr.Code),
			ExitCode: exit,
			Details: map[string]any{
				"synology_code": dsErr.Code,
			},
			Err: err,
		}
	}
	var app *apperr.Error
	if errors.As(err, &app) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperr.Wrap("timeout", "command timed out", 5, err)
	}
	return apperr.Wrap("internal_error", "command failed", 1, err)
}

func (a *appContext) outputError(commandName, endpoint string, start time.Time, err error) error {
	if !a.opts.JSON {
		return err
	}
	env := output.NewEnvelope(false, commandName, endpoint, start)
	env.Error = &output.ErrInfo{
		Code:    apperr.Code(err),
		Message: err.Error(),
		Details: apperr.Details(err),
	}
	_ = output.WriteJSON(a.out, env)
	return err
}

func joinCommand(name ...string) string {
	return strings.Join(name, " ")
}

func (a *appContext) validateAuthOptions() error {
	if a.opts.CredentialsFile != "" {
		if a.opts.User != "" || a.opts.Password != "" || a.opts.PasswordStdin {
			return errors.New("use --credentials-file without --user, --password, or --password-stdin")
		}
		return nil
	}
	if a.opts.Password != "" && a.opts.PasswordStdin {
		return errors.New("use only one of --password or --password-stdin")
	}
	return nil
}

func clampVersion(version, maxSupported int) int {
	if version <= 0 {
		return maxSupported
	}
	if version > maxSupported {
		return maxSupported
	}
	return version
}

func selectDownloadStationAPIs(entries map[string]apiinfo.Entry) (taskName, taskPath string, taskVersion int, listName, listPath string, listVersion int) {
	taskName, taskPath, taskVersion = "SYNO.DownloadStation.Task", "/webapi/DownloadStation/task.cgi", 1
	type candidate struct {
		name   string
		path   string
		min    int
		max    int
		suffix int
	}
	var taskCandidates []candidate
	var listCandidates []candidate
	for name, entry := range entries {
		if m := taskAPIRe.FindStringSubmatch(name); m != nil {
			suffix := 0
			if m[1] != "" {
				fmt.Sscanf(m[1], "%d", &suffix)
			}
			taskCandidates = append(taskCandidates, candidate{name: name, path: "/webapi/" + entry.Path, min: entry.MinVersion, max: entry.MaxVersion, suffix: suffix})
			continue
		}
		if m := taskListAPIRe.FindStringSubmatch(name); m != nil {
			suffix := 0
			if m[1] != "" {
				fmt.Sscanf(m[1], "%d", &suffix)
			}
			listCandidates = append(listCandidates, candidate{name: name, path: "/webapi/" + entry.Path, min: entry.MinVersion, max: entry.MaxVersion, suffix: suffix})
		}
	}
	sort.Slice(taskCandidates, func(i, j int) bool {
		if taskCandidates[i].suffix != taskCandidates[j].suffix {
			return taskCandidates[i].suffix > taskCandidates[j].suffix
		}
		return taskCandidates[i].max > taskCandidates[j].max
	})
	if len(taskCandidates) > 0 {
		best := taskCandidates[0]
		taskName, taskPath, taskVersion = best.name, best.path, best.max
	}
	sort.Slice(listCandidates, func(i, j int) bool {
		if listCandidates[i].suffix != listCandidates[j].suffix {
			return listCandidates[i].suffix > listCandidates[j].suffix
		}
		return listCandidates[i].max > listCandidates[j].max
	})
	if len(listCandidates) > 0 {
		best := listCandidates[0]
		listName, listPath, listVersion = best.name, best.path, best.max
	}
	return taskName, taskPath, taskVersion, listName, listPath, listVersion
}

func printTable(w io.Writer, headers []string, rows [][]string) {
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
}
