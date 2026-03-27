package cli

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"synocli/internal/update"
)

type countingDoer struct {
	calls int32
}

func (d *countingDoer) Do(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&d.calls, 1)
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
	}, nil
}

func TestBackgroundUpdateCheckSkipByFlag(t *testing.T) {
	withBuildInfo("v0.1.0", "abc1234", "2026-03-27T00:00:00Z", func() {
		var out, errOut bytes.Buffer
		doer := &countingDoer{}
		prev := buildUpdateClient
		buildUpdateClient = func(timeout time.Duration) *update.Client {
			return update.NewClient(doer)
		}
		defer func() { buildUpdateClient = prev }()

		cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
		cmd.SetArgs([]string{"--no-update-check", "version"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if got := atomic.LoadInt32(&doer.calls); got != 0 {
			t.Fatalf("unexpected update checks: %d", got)
		}
	})
}

func TestBackgroundUpdateCheckSkipByJSON(t *testing.T) {
	withBuildInfo("v0.1.0", "abc1234", "2026-03-27T00:00:00Z", func() {
		var out, errOut bytes.Buffer
		doer := &countingDoer{}
		prev := buildUpdateClient
		buildUpdateClient = func(timeout time.Duration) *update.Client {
			return update.NewClient(doer)
		}
		defer func() { buildUpdateClient = prev }()

		cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
		cmd.SetArgs([]string{"--json", "version"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if got := atomic.LoadInt32(&doer.calls); got != 0 {
			t.Fatalf("unexpected update checks: %d", got)
		}
	})
}
