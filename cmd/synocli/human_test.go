package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewHumanUINonTTY(t *testing.T) {
	var out bytes.Buffer
	ui := newHumanUI(&out)
	if ui.tty {
		t.Fatal("expected non-tty writer")
	}
	if ui.styled {
		t.Fatal("expected styling disabled for non-tty writer")
	}
}

func TestPrintErrorNoANSIOnBuffer(t *testing.T) {
	var out bytes.Buffer
	printError(&out, errors.New("boom"))
	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("unexpected ANSI output: %q", got)
	}
	if !strings.Contains(got, "ERROR boom") {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestPrintWatchHeaderIncludesFilters(t *testing.T) {
	var out bytes.Buffer
	printWatchHeader(&out, time.Unix(0, 0).UTC(), 3, []string{"dbid_1"}, []string{"downloading"})
	got := out.String()
	for _, want := range []string{"Download Station Watch", "Tasks", "3", "dbid_1", "downloading"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q: %q", want, got)
		}
	}
}
