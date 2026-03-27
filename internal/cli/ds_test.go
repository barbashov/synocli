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

func TestDSIncludesCleanupCommand(t *testing.T) {
	cmd := newDSCmd(&appContext{})
	cleanup, _, err := cmd.Find([]string{"cleanup"})
	if err != nil {
		t.Fatalf("find cleanup: %v", err)
	}
	if cleanup == nil || cleanup.Name() != "cleanup" {
		t.Fatalf("cleanup command not found: %#v", cleanup)
	}
}

func TestDSCleanupFlags(t *testing.T) {
	cmd := newDSCleanupCmd(&appContext{})
	include := cmd.Flags().Lookup("include-seeding")
	if include == nil || include.Shorthand != "s" {
		t.Fatalf("expected --include-seeding shorthand -s, got %#v", include)
	}
	yes := cmd.Flags().Lookup("yes")
	if yes == nil || yes.Shorthand != "y" {
		t.Fatalf("expected --yes shorthand -y, got %#v", yes)
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

func TestCleanupTaskIDsFiltersNormalizedStatuses(t *testing.T) {
	tasks := []downloadstation.Task{
		{ID: "a", Status: "finished"},
		{ID: "b", Status: "seeding"},
		{ID: "c", Status: "downloading"},
		{ID: "d", Status: "5"},
		{ID: "e", Status: "7"},
	}
	finishedOnly := cleanupTaskIDs(cleanupTasks(tasks, cleanupStatuses(false)))
	if got := strings.Join(finishedOnly, ","); got != "a,d" {
		t.Fatalf("finished-only ids=%q want a,d", got)
	}
	withSeeding := cleanupTaskIDs(cleanupTasks(tasks, cleanupStatuses(true)))
	if got := strings.Join(withSeeding, ","); got != "a,b,d,e" {
		t.Fatalf("finished+seeding ids=%q want a,b,d,e", got)
	}
}

func TestIsAffirmativeAnswer(t *testing.T) {
	if !isAffirmativeAnswer("y") || !isAffirmativeAnswer("YES\n") {
		t.Fatal("expected y/yes to be accepted")
	}
	if isAffirmativeAnswer("n") || isAffirmativeAnswer("") || isAffirmativeAnswer("ok") {
		t.Fatal("unexpected affirmative parse")
	}
}

func TestPromptCleanupConfirmationRejectsNonTTY(t *testing.T) {
	var out bytes.Buffer
	confirmed, err := promptCleanupConfirmation(strings.NewReader("yes\n"), &out, []downloadstation.Task{{ID: "a"}}, cleanupStatuses(true))
	if err == nil {
		t.Fatal("expected non-tty error")
	}
	if confirmed {
		t.Fatal("expected confirmed=false on non-tty")
	}
	if !strings.Contains(err.Error(), "pass --yes (-y)") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintCleanupPreviewUsesListStyleTable(t *testing.T) {
	var out bytes.Buffer
	tasks := []downloadstation.Task{
		{
			ID:     "dbid_1",
			Title:  "ubuntu.iso",
			Status: "finished",
			Type:   "bt",
		},
	}
	printCleanupPreview(&out, tasks, cleanupStatuses(true))
	got := out.String()
	for _, want := range []string{"Downloads", "Tasks", "Status Filter", "ID", "Title", "Status", "finished,seeding", "dbid_1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("cleanup preview missing %q: %q", want, got)
		}
	}
}

func TestNormalizeFailedTaskIDs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "single", in: "dbid_1", want: "dbid_1"},
		{name: "comma", in: "dbid_1,dbid_2", want: "dbid_1,dbid_2"},
		{name: "json_array", in: `["dbid_1","dbid_2"]`, want: "dbid_1,dbid_2"},
		{name: "empty", in: "  ", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(normalizeFailedTaskIDs(tc.in), ",")
			if got != tc.want {
				t.Fatalf("normalizeFailedTaskIDs(%q)=%q want %q", tc.in, got, tc.want)
			}
		})
	}
}
