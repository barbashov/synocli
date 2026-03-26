package main

import (
	"strings"
	"testing"
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
			cmd := newDSWatchCmd(ac)
			cmd.SetArgs([]string{"--interval", interval})
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
