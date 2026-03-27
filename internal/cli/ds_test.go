package cli

import (
	"bytes"
	"strings"
	"testing"

	"synocli/internal/synology/downloadstation"
)

func TestDSCommandAliases(t *testing.T) {
	cmd := newDSCmd(&appContext{})
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "downloadstation" {
		t.Fatalf("expected downloadstation alias, got %#v", cmd.Aliases)
	}
}

func TestWaitRejectsNonPositiveInterval(t *testing.T) {
	tests := []string{"0s", "-1s"}
	for _, interval := range tests {
		t.Run(interval, func(t *testing.T) {
			ac := &appContext{}
			cmd := newDSWaitCmd(ac)
			cmd.SetArgs([]string{"dbid_1", "--interval", interval})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "--interval must be greater than 0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWatchRejectsNonPositiveInterval(t *testing.T) {
	tests := []string{"0s", "-1s"}
	for _, interval := range tests {
		t.Run(interval, func(t *testing.T) {
			ac := &appContext{}
			cmd := newDSListCmd(ac)
			cmd.SetArgs([]string{"--watch", "--interval", interval})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "--interval must be greater than 0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFormatTaskETA(t *testing.T) {
	known := downloadstation.Task{
		Size: 1000,
		Additional: &downloadstation.TaskAdditional{
			Transfer: &downloadstation.TaskTransfer{SizeDownloaded: 400, SpeedDownload: 100},
		},
	}
	if got := formatTaskETA(known); got != "6 seconds" {
		t.Fatalf("formatTaskETA known=%q want %q", got, "6 seconds")
	}

	unknown := downloadstation.Task{
		Size: 1000,
		Additional: &downloadstation.TaskAdditional{
			Transfer: &downloadstation.TaskTransfer{SizeDownloaded: 400, SpeedDownload: 0},
		},
	}
	if got := formatTaskETA(unknown); got != "-" {
		t.Fatalf("formatTaskETA unknown=%q want -", got)
	}
}

func TestPrintTaskTableIncludesETAColumn(t *testing.T) {
	var out bytes.Buffer
	tasks := []downloadstation.Task{
		{
			ID:     "dbid_1",
			Title:  "ubuntu.iso",
			Status: "downloading",
			Type:   "bt",
			Size:   1000,
			Additional: &downloadstation.TaskAdditional{
				Detail:   &downloadstation.TaskDetail{Destination: "/volume1/downloads"},
				Transfer: &downloadstation.TaskTransfer{SizeDownloaded: 400, SpeedDownload: 100},
			},
		},
	}
	printTaskTable(&out, tasks)
	got := out.String()
	if !strings.Contains(got, "ETA") {
		t.Fatalf("table missing ETA header: %q", got)
	}
	if !strings.Contains(got, "6 seconds") {
		t.Fatalf("table missing ETA value: %q", got)
	}
}
