package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFSCopyRequiresDestinationFlag(t *testing.T) {
	ac := &appContext{}
	cmd := newFSCopyCmd(ac, false)
	cmd.SetArgs([]string{"/src/path"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--to is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFSSearchRequiresPattern(t *testing.T) {
	ac := &appContext{}
	cmd := newFSSearchCmd(ac)
	cmd.SetArgs([]string{"/"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--pattern is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFSSearchClearRequiresTaskID(t *testing.T) {
	ac := &appContext{}
	searchCmd := newFSSearchCmd(ac)
	searchCmd.SetArgs([]string{"clear"})
	err := searchCmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requires at least 1 arg(s)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootIncludesFSAlias(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
	cmd.SetArgs([]string{"filestation", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFSListAliases(t *testing.T) {
	cmd := newFSListCmd(&appContext{})
	if len(cmd.Aliases) == 0 || cmd.Aliases[0] != "ls" {
		t.Fatalf("expected ls alias, got %#v", cmd.Aliases)
	}
}

func TestFSCopyMoveAliases(t *testing.T) {
	copyCmd := newFSCopyCmd(&appContext{}, false)
	moveCmd := newFSCopyCmd(&appContext{}, true)
	if len(copyCmd.Aliases) == 0 || copyCmd.Aliases[0] != "cp" {
		t.Fatalf("expected cp alias, got %#v", copyCmd.Aliases)
	}
	if len(moveCmd.Aliases) == 0 || moveCmd.Aliases[0] != "mv" {
		t.Fatalf("expected mv alias, got %#v", moveCmd.Aliases)
	}
}

func TestFSWatchFlags(t *testing.T) {
	tasksCmd := newFSTasksCmd(&appContext{})
	if tasksCmd.Flags().Lookup("watch") == nil {
		t.Fatal("fs tasks missing --watch flag")
	}
	if tasksCmd.Flags().Lookup("interval") == nil {
		t.Fatal("fs tasks missing --interval flag")
	}
	listCmd := newFSListCmd(&appContext{})
	if listCmd.Flags().Lookup("watch") == nil {
		t.Fatal("fs list missing --watch flag")
	}
	if listCmd.Flags().Lookup("interval") == nil {
		t.Fatal("fs list missing --interval flag")
	}
}

func TestFSListSizeAndMTimeDisplay(t *testing.T) {
	file := map[string]any{
		"isdir": false,
		"size":  float64(1536),
		"time":  map[string]any{"mtime": float64(time.Now().Unix())},
	}
	if got := fsListSizeDisplay(file); got != "1.5 KB" {
		t.Fatalf("size=%q want 1.5 KB", got)
	}
	if got := fsListMTimeDisplay(file); got == "-" || got == "" {
		t.Fatalf("mtime should be formatted, got %q", got)
	}

	dir := map[string]any{"isdir": true}
	if got := fsListSizeDisplay(dir); got != "<DIR>" {
		t.Fatalf("dir size=%q want <DIR>", got)
	}
}

func TestFSFileSizeFallbackFromAdditional(t *testing.T) {
	file := map[string]any{
		"additional": map[string]any{
			"size": float64(2048),
			"time": map[string]any{"mtime": float64(1700000000)},
		},
	}
	if got := fsFileSize(file); got != 2048 {
		t.Fatalf("size=%d want 2048", got)
	}
	if got := fsFileMTime(file); got != 1700000000 {
		t.Fatalf("mtime=%d want 1700000000", got)
	}
}
