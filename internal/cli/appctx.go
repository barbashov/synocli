package cli

import (
	"io"
	"strings"
	"time"

	"synocli/internal/config"
	"synocli/internal/synology/auth"
	"synocli/internal/synology/downloadstation"
	"synocli/internal/synology/filestation"
	"synocli/internal/synology/storage"
	"synocli/internal/synology/system"
)

type appContext struct {
	opts  config.GlobalOptions
	stdin io.Reader
	out   io.Writer
	err   io.Writer
}

type session struct {
	endpoint      string
	start         time.Time
	authClient    *auth.Client
	dsClient      *downloadstation.Client
	fsClient      *filestation.Client
	sysClient     *system.Client
	storageClient *storage.Client
	apiVersions   map[string]int
	committed     bool
}

// markCommitted records that the command closure has performed a server-side
// mutation. After this point withSession will not re-run the closure on a
// session-expiry re-login, so non-idempotent work is never duplicated.
func (s *session) markCommitted() {
	s.committed = true
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
