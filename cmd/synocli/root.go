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
	"synocli/internal/synology/filestation"
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
	fsClient    *filestation.Client
	apiVersions map[string]int
}

type jsonOutputHandledError struct {
	err error
}

func (e *jsonOutputHandledError) Error() string {
	return e.err.Error()
}

func (e *jsonOutputHandledError) Unwrap() error {
	return e.err
}

var taskAPIRe = regexp.MustCompile(`^SYNO\.DownloadStation(\d*)\.Task$`)

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
	f.BoolVar(&ac.opts.Debug, "debug", false, "Debug request flow")

	cmd.AddCommand(newAuthCmd(ac), newDSCmd(ac), newFSCmd(ac), newCLIConfigCmd(ac), newVersionCmd(ac))
	return cmd
}

func (a *appContext) withSession(cmd *cobra.Command, commandName string, fn func(context.Context, *session) (any, error)) error {
	start := time.Now()
	runOpts, err := a.resolveRuntimeOptions(cmd)
	if err != nil {
		return a.outputError(commandName, "", start, apperr.Wrap("validation_error", "invalid runtime options", 1, err))
	}
	if runOpts.Endpoint == "" {
		return a.outputError(commandName, "", start, apperr.New("validation_error", "endpoint is required via --endpoint or config file", 1))
	}
	u, err := config.ValidateEndpoint(runOpts.Endpoint)
	if err != nil {
		return a.outputError(commandName, runOpts.Endpoint, start, apperr.Wrap("validation_error", "invalid endpoint", 1, err))
	}
	if err := runOpts.ResolvePassword(a.stdin); err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("validation_error", "invalid auth options", 1, err))
	}
	a.opts = runOpts

	hc, err := httpclient.New(httpclient.Options{InsecureTLS: runOpts.InsecureTLS, Timeout: runOpts.Timeout, Debug: runOpts.Debug, DebugOut: a.err})
	if err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("internal_error", "initialize http client", 1, err))
	}
	ctx := cmd.Context()

	entries, err := apiinfo.Discover(ctx, u.String(), hc)
	if err != nil {
		entries = map[string]apiinfo.Entry{}
	}
	authPath, authVersion := apiinfo.Select(entries, "SYNO.API.Auth", "/webapi/auth.cgi", 6)
	authVersion = clampVersion(authVersion, 6)

	dsAPIName, dsPath, dsVersion := selectDownloadStationAPIs(entries)
	dsVersion = clampVersion(dsVersion, 3)
	if runOpts.Debug {
		_, _ = fmt.Fprintf(a.err, "[debug] selected task api=%s path=%s version=%d\n", dsAPIName, dsPath, dsVersion)
	}

	fsAPIs := selectFileStationAPIs(entries)

	apiVersions := map[string]int{"auth": authVersion, "task": dsVersion}
	for key, api := range fsAPIs {
		apiVersions["fs_"+key] = api.Version
	}

	authClient := &auth.Client{Endpoint: u.String(), Path: authPath, Version: authVersion, HTTP: hc}
	sid, err := authClient.Login(ctx, runOpts.User, runOpts.Password, "synocli")
	if err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("auth_failed", "authentication failed", 2, err))
	}
	defer func() {
		_ = authClient.Logout(context.Background(), sid, "synocli")
	}()

	dsClient, err := downloadstation.NewClient(u.String(), sid, hc, dsPath, dsVersion, dsAPIName)
	if err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("internal_error", "initialize download station client", 1, err))
	}
	fsClient, err := filestation.NewClient(u.String(), sid, hc, fsAPIs)
	if err != nil {
		return a.outputError(commandName, u.String(), start, apperr.Wrap("internal_error", "initialize file station client", 1, err))
	}

	s := &session{
		endpoint:    u.String(),
		start:       start,
		authClient:  authClient,
		dsClient:    dsClient,
		fsClient:    fsClient,
		apiVersions: apiVersions,
	}
	data, err := fn(ctx, s)
	if err != nil {
		return a.outputError(commandName, u.String(), start, toAppError(err))
	}
	if data == nil {
		return nil
	}
	env := output.NewEnvelope(true, commandName, u.String(), start)
	env.Data = data
	env.Meta.APIVersion = s.apiVersions
	if err := output.WriteJSON(a.out, env); err != nil {
		return apperr.Wrap("internal_error", "write json output", 1, err)
	}
	return nil
}

func toAppError(err error) error {
	var dsErr *downloadstation.APIError
	if errors.As(err, &dsErr) {
		code := "synology_error"
		exit := 1
		if dsErr.Code == 404 {
			exit = 3
		}
		details := map[string]any{
			"synology_code": dsErr.Code,
		}
		if len(dsErr.FailedTasks) > 0 {
			failed := make([]map[string]any, 0, len(dsErr.FailedTasks))
			ids := make([]string, 0, len(dsErr.FailedTasks))
			for _, ft := range dsErr.FailedTasks {
				failed = append(failed, map[string]any{
					"id":   ft.ID,
					"code": ft.Code,
				})
				if ft.ID != "" {
					ids = append(ids, ft.ID)
				}
			}
			details["failed_tasks"] = failed
			if len(ids) > 0 {
				details["failed_task_ids"] = ids
			}
		}
		return &apperr.Error{
			Code:     code,
			Message:  downloadstation.ErrorMessage(dsErr.Code),
			ExitCode: exit,
			Details:  details,
			Err:      err,
		}
	}
	var fsErr *filestation.APIError
	if errors.As(err, &fsErr) {
		code := fsErr.EffectiveCode()
		details := map[string]any{
			"synology_code": code,
		}
		if fsErr.Path != "" {
			details["path"] = fsErr.Path
		}
		if fsErr.Code != 0 && fsErr.Code != code {
			details["synology_parent_code"] = fsErr.Code
		}
		return &apperr.Error{
			Code:     "synology_error",
			Message:  filestation.ErrorMessage(code),
			ExitCode: 1,
			Details:  details,
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
	return &jsonOutputHandledError{err: err}
}

func joinCommand(name ...string) string {
	return strings.Join(name, " ")
}

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

func clampVersion(version, maxSupported int) int {
	if version <= 0 {
		return maxSupported
	}
	if version > maxSupported {
		return maxSupported
	}
	return version
}

func selectDownloadStationAPIs(entries map[string]apiinfo.Entry) (taskName, taskPath string, taskVersion int) {
	taskName, taskPath, taskVersion = "SYNO.DownloadStation.Task", "/webapi/DownloadStation/task.cgi", 1
	type candidate struct {
		name   string
		path   string
		min    int
		max    int
		suffix int
	}
	var taskCandidates []candidate
	for name, entry := range entries {
		if m := taskAPIRe.FindStringSubmatch(name); m != nil {
			suffix := 0
			if m[1] != "" {
				_, _ = fmt.Sscanf(m[1], "%d", &suffix)
			}
			taskCandidates = append(taskCandidates, candidate{name: name, path: "/webapi/" + entry.Path, min: entry.MinVersion, max: entry.MaxVersion, suffix: suffix})
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
	return taskName, taskPath, taskVersion
}

func selectFileStationAPIs(entries map[string]apiinfo.Entry) map[string]filestation.APISpec {
	type fsAPI struct {
		key          string
		apiName      string
		fallbackVer  int
		maxSupported int
	}
	catalog := []fsAPI{
		{key: filestation.APIInfo, apiName: "SYNO.FileStation.Info", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APIList, apiName: "SYNO.FileStation.List", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APICreateFolder, apiName: "SYNO.FileStation.CreateFolder", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APIRename, apiName: "SYNO.FileStation.Rename", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APIDelete, apiName: "SYNO.FileStation.Delete", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APICopyMove, apiName: "SYNO.FileStation.CopyMove", fallbackVer: 3, maxSupported: 3},
		{key: filestation.APIUpload, apiName: "SYNO.FileStation.Upload", fallbackVer: 2, maxSupported: 3},
		{key: filestation.APIDownload, apiName: "SYNO.FileStation.Download", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APISearch, apiName: "SYNO.FileStation.Search", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APIDirSize, apiName: "SYNO.FileStation.DirSize", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APIMD5, apiName: "SYNO.FileStation.MD5", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APIExtract, apiName: "SYNO.FileStation.Extract", fallbackVer: 2, maxSupported: 2},
		{key: filestation.APICompress, apiName: "SYNO.FileStation.Compress", fallbackVer: 3, maxSupported: 3},
		{key: filestation.APIBackgroundTask, apiName: "SYNO.FileStation.BackgroundTask", fallbackVer: 3, maxSupported: 3},
	}
	out := make(map[string]filestation.APISpec, len(catalog))
	for _, item := range catalog {
		path, version := apiinfo.Select(entries, item.apiName, "/webapi/entry.cgi", item.fallbackVer)
		version = clampVersion(version, item.maxSupported)
		out[item.key] = filestation.APISpec{Name: item.apiName, Path: path, Version: version}
	}
	return out
}
