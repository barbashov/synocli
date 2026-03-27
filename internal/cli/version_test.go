package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"synocli/internal/output"
)

func withBuildInfo(version, commit, date string, fn func()) {
	prevVersion, prevCommit, prevDate := buildVersion, buildCommit, buildDate
	buildVersion, buildCommit, buildDate = version, commit, date
	defer func() {
		buildVersion, buildCommit, buildDate = prevVersion, prevCommit, prevDate
	}()
	fn()
}

func TestRootVersionFlag(t *testing.T) {
	withBuildInfo("v0.1.0", "abc1234", "2026-03-27T00:00:00Z", func() {
		var out, errOut bytes.Buffer
		cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
		cmd.SetArgs([]string{"--no-update-check", "--version"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if got := strings.TrimSpace(out.String()); got != "v0.1.0" {
			t.Fatalf("unexpected version output: %q", got)
		}
		if errOut.Len() != 0 {
			t.Fatalf("unexpected stderr output: %q", errOut.String())
		}
	})
}

func TestVersionCommandHuman(t *testing.T) {
	withBuildInfo("v0.1.0", "abc1234", "2026-03-27T00:00:00Z", func() {
		var out, errOut bytes.Buffer
		cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
		cmd.SetArgs([]string{"--no-update-check", "version"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		got := out.String()
		for _, want := range []string{"synocli v0.1.0", "abc1234", "2026-03-27T00:00:00Z"} {
			if !strings.Contains(got, want) {
				t.Fatalf("output missing %q: %q", want, got)
			}
		}
		if errOut.Len() != 0 {
			t.Fatalf("unexpected stderr output: %q", errOut.String())
		}
	})
}

func TestVersionCommandJSON(t *testing.T) {
	withBuildInfo("v0.1.0", "abc1234", "2026-03-27T00:00:00Z", func() {
		var out, errOut bytes.Buffer
		cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
		cmd.SetArgs([]string{"--no-update-check", "--json", "version"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		var env output.Envelope
		if err := json.Unmarshal(out.Bytes(), &env); err != nil {
			t.Fatalf("unmarshal output: %v; raw=%q", err, out.String())
		}
		if !env.OK || env.Command != "version" {
			t.Fatalf("unexpected envelope: %+v", env)
		}
		data, ok := env.Data.(map[string]any)
		if !ok {
			t.Fatalf("unexpected data type: %T", env.Data)
		}
		if data["version"] != "v0.1.0" || data["commit"] != "abc1234" || data["build_date"] != "2026-03-27T00:00:00Z" {
			t.Fatalf("unexpected data: %#v", data)
		}
		if errOut.Len() != 0 {
			t.Fatalf("unexpected stderr output: %q", errOut.String())
		}
	})
}
