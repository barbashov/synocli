package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
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

const synologySession = "synocli"

var taskAPIRe = regexp.MustCompile(`^SYNO\.DownloadStation(\d*)\.Task$`)

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

	var sessionPath string
	logoutSID := ""
	defer func() {
		if logoutSID != "" {
			_ = authClient.Logout(context.Background(), logoutSID, synologySession)
		}
	}()

	// loginAndSave resolves credentials if not yet done, logs in, and persists
	// the new SID when reuse_session is enabled. It updates logoutSID so the
	// deferred cleanup knows whether to call Logout on exit.
	loginAndSave := func() (string, error) {
		if runOpts.Password == "" {
			if err := runOpts.ResolvePassword(a.stdin); err != nil {
				return "", a.outputError(commandName, u.String(), start, apperr.Wrap("validation_error", "invalid auth options", 1, err))
			}
		}
		if runOpts.User == "" {
			return "", a.outputError(commandName, u.String(), start, apperr.New("validation_error", "--user is required", 1))
		}
		newSID, loginErr := authClient.Login(ctx, runOpts.User, runOpts.Password, synologySession)
		if loginErr != nil {
			return "", a.outputError(commandName, u.String(), start, apperr.Wrap("auth_failed", "authentication failed", 2, loginErr))
		}
		if runOpts.ReuseSession {
			if writeErr := config.WriteSession(sessionPath, newSID); writeErr != nil {
				if runOpts.Debug {
					_, _ = fmt.Fprintf(a.err, "[debug] write session: %v\n", writeErr)
				}
				logoutSID = newSID
			} else {
				logoutSID = ""
			}
		} else {
			logoutSID = newSID
		}
		return newSID, nil
	}

	var sid string
	if runOpts.ReuseSession {
		sessionPath = config.SessionPathFromConfig(runOpts.ConfigPath)
		if cached, loadErr := config.LoadSession(sessionPath); loadErr != nil {
			if runOpts.Debug {
				_, _ = fmt.Fprintf(a.err, "[debug] load session: %v\n", loadErr)
			}
		} else {
			sid = cached
		}
	}

	if sid == "" {
		var err error
		sid, err = loginAndSave()
		if err != nil {
			return err
		}
	}
	a.opts = runOpts

	runWithSID := func(id string) (any, error) {
		dsClient, err := downloadstation.NewClient(u.String(), id, hc, dsPath, dsVersion, dsAPIName)
		if err != nil {
			return nil, apperr.Wrap("internal_error", "initialize download station client", 1, err)
		}
		fsClient, err := filestation.NewClient(u.String(), id, hc, fsAPIs)
		if err != nil {
			return nil, apperr.Wrap("internal_error", "initialize file station client", 1, err)
		}
		return fn(ctx, &session{
			endpoint:    u.String(),
			start:       start,
			authClient:  authClient,
			dsClient:    dsClient,
			fsClient:    fsClient,
			apiVersions: apiVersions,
		})
	}

	data, fnErr := runWithSID(sid)

	if fnErr != nil && runOpts.ReuseSession && isSessionExpiry(fnErr) {
		_ = config.DeleteSession(sessionPath)
		newSID, err := loginAndSave()
		if err != nil {
			return err
		}
		a.opts = runOpts
		data, fnErr = runWithSID(newSID)
	}

	if fnErr != nil {
		return a.outputError(commandName, u.String(), start, toAppError(fnErr))
	}
	if data == nil {
		return nil
	}
	env := output.NewEnvelope(true, commandName, u.String(), start)
	env.Data = data
	env.Meta.APIVersion = apiVersions
	if err := output.WriteJSON(a.out, env); err != nil {
		return apperr.Wrap("internal_error", "write json output", 1, err)
	}
	return nil
}

func isSessionExpiry(err error) bool {
	var dsErr *downloadstation.APIError
	if errors.As(err, &dsErr) {
		return dsErr.Code == 106 || dsErr.Code == 107 || dsErr.Code == 119
	}
	var fsErr *filestation.APIError
	if errors.As(err, &fsErr) {
		c := fsErr.EffectiveCode()
		return c == 106 || c == 107 || c == 119
	}
	return false
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
