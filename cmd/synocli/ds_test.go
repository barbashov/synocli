package main

import (
	"strings"
	"testing"

	"synocli/internal/synology/downloadstation"
)

func TestMapTaskIncludesStatusEnumFields(t *testing.T) {
	m := mapTask(downloadstation.Task{
		ID:     "dbid_1",
		Title:  "x",
		Status: "3",
	})
	if got := m["raw_status"]; got != "3" {
		t.Fatalf("raw_status=%v want 3", got)
	}
	if got := m["status_enum"]; got != "paused" {
		t.Fatalf("status_enum=%v want paused", got)
	}
	if got := m["status_display"]; got != "paused (3)" {
		t.Fatalf("status_display=%v want paused (3)", got)
	}
	code, ok := m["status_code"].(int)
	if !ok || code != 3 {
		t.Fatalf("status_code=%v want int(3)", m["status_code"])
	}
}

func TestWaitRejectsNonPositiveInterval(t *testing.T) {
	tests := []string{"0s", "-1s"}
	for _, interval := range tests {
		t.Run(interval, func(t *testing.T) {
			ac := &appContext{}
			cmd := newDSWaitCmd(ac)
			cmd.SetArgs([]string{"https://example.com:5001", "dbid_1", "--interval", interval})
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
			cmd := newDSWatchCmd(ac)
			cmd.SetArgs([]string{"https://example.com:5001", "--interval", interval})
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
