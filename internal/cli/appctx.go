package cli

import (
	"io"
	"strings"
	"time"

	"synocli/internal/config"
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

func joinCommand(name ...string) string {
	return strings.Join(name, " ")
}
