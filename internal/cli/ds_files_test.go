package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"synocli/internal/synology/downloadstation"
)

func TestDSIncludesFilesCommand(t *testing.T) {
	cmd := newDSCmd(&appContext{})
	for _, path := range [][]string{
		{"files"},
		{"files", "list"},
		{"files", "ls"}, // alias
		{"files", "set"},
		{"files", "priority"},
	} {
		found, _, err := cmd.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if found == nil {
			t.Fatalf("command %v not found", path)
		}
	}
}

func TestDSFilesSetFlags(t *testing.T) {
	cmd := newDSFilesSetCmd(&appContext{})
	for _, name := range []string{"include", "skip", "all", "none"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}
}

func TestDSFilesPriorityFlags(t *testing.T) {
	cmd := newDSFilesPriorityCmd(&appContext{})
	for _, name := range []string{"index", "all"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}
}

func TestValidateWantedFlags(t *testing.T) {
	tests := []struct {
		name    string
		all     bool
		none    bool
		include []int
		skip    []int
		wantErr bool
	}{
		{name: "include only", include: []int{0}, wantErr: false},
		{name: "skip only", skip: []int{1}, wantErr: false},
		{name: "include and skip", include: []int{0}, skip: []int{1}, wantErr: false},
		{name: "all only", all: true, wantErr: false},
		{name: "none only", none: true, wantErr: false},
		{name: "no flags", wantErr: true},
		{name: "all and none", all: true, none: true, wantErr: true},
		{name: "all with include", all: true, include: []int{0}, wantErr: true},
		{name: "none with skip", none: true, skip: []int{0}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWantedFlags(tc.all, tc.none, tc.include, tc.skip)
			if tc.wantErr != (err != nil) {
				t.Fatalf("validateWantedFlags err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestResolveWantedSelection(t *testing.T) {
	files := []downloadstation.BTFile{{Index: 0}, {Index: 1}, {Index: 2}}

	t.Run("all", func(t *testing.T) {
		want, skip, err := resolveWantedSelection(files, true, false, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, []int64{0, 1, 2}) || len(skip) != 0 {
			t.Fatalf("want=%v skip=%v", want, skip)
		}
	})

	t.Run("none", func(t *testing.T) {
		want, skip, err := resolveWantedSelection(files, false, true, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(want) != 0 || !reflect.DeepEqual(skip, []int64{0, 1, 2}) {
			t.Fatalf("want=%v skip=%v", want, skip)
		}
	})

	t.Run("include and skip sorted and deduped", func(t *testing.T) {
		want, skip, err := resolveWantedSelection(files, false, false, []int{2, 0, 0}, []int{1})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, []int64{0, 2}) || !reflect.DeepEqual(skip, []int64{1}) {
			t.Fatalf("want=%v skip=%v", want, skip)
		}
	})

	t.Run("out-of-range index", func(t *testing.T) {
		_, _, err := resolveWantedSelection(files, false, false, []int{5}, nil)
		if err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("expected out-of-range error, got %v", err)
		}
	})

	t.Run("index in both include and skip", func(t *testing.T) {
		_, _, err := resolveWantedSelection(files, false, false, []int{1}, []int{1})
		if err == nil || !strings.Contains(err.Error(), "both --include and --skip") {
			t.Fatalf("expected conflict error, got %v", err)
		}
	})
}

func TestResolvePriorityIndices(t *testing.T) {
	files := []downloadstation.BTFile{{Index: 0}, {Index: 1}}

	all, err := resolvePriorityIndices(files, true, nil)
	if err != nil || !reflect.DeepEqual(all, []int64{0, 1}) {
		t.Fatalf("all: %v err=%v", all, err)
	}

	some, err := resolvePriorityIndices(files, false, []int{1})
	if err != nil || !reflect.DeepEqual(some, []int64{1}) {
		t.Fatalf("some: %v err=%v", some, err)
	}

	if _, err := resolvePriorityIndices(files, false, []int{9}); err == nil {
		t.Fatal("expected out-of-range error")
	}
}

func TestIsValidPriority(t *testing.T) {
	for _, p := range []string{"low", "normal", "high"} {
		if !isValidPriority(p) {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []string{"skip", "", "HIGH", "turbo"} {
		if isValidPriority(p) {
			t.Errorf("%q should be invalid", p)
		}
	}
}

func TestPrintBTFileTable(t *testing.T) {
	var buf bytes.Buffer
	printBTFileTable(&buf, []downloadstation.BTFile{
		{Index: 0, Name: "movie.mkv", Size: 1000, SizeDownloaded: 500, Priority: "high", Wanted: true},
		{Index: 1, Name: "sample.txt", Size: 100, SizeDownloaded: 0, Priority: "normal", Wanted: false},
	})
	out := buf.String()
	for _, want := range []string{"Index", "Name", "Size", "Downloaded", "Progress", "Wanted", "Priority", "movie.mkv", "sample.txt", "yes", "no", "high"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}
}

func TestJoinInts(t *testing.T) {
	if got := joinInts(nil); got != "-" {
		t.Errorf("joinInts(nil) = %q, want -", got)
	}
	if got := joinInts([]int64{0, 2, 5}); got != "0, 2, 5" {
		t.Errorf("joinInts = %q", got)
	}
}
