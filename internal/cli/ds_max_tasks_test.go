package cli

import (
	"strings"
	"testing"
)

func TestDSMaxTasksHasGetAndSet(t *testing.T) {
	cmd := newDSMaxTasksCmd(&appContext{})
	for _, name := range []string{"get", "set"} {
		sub, _, err := cmd.Find([]string{name})
		if err != nil || sub == nil || sub.Name() != name {
			t.Fatalf("max-tasks %s not found: sub=%#v err=%v", name, sub, err)
		}
	}
}

func TestDSMaxTasksSetRequiresArg(t *testing.T) {
	cmd := newDSMaxTasksSetCmd(&appContext{})
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no positional arg provided")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDSMaxTasksSetRejectsBadValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"zero", []string{"0"}, "must be >= 1"},
		// Use "--" so Cobra doesn't try to parse the leading "-" as a flag.
		{"negative", []string{"--", "-3"}, "must be >= 1"},
		{"non-numeric", []string{"abc"}, "must be an integer"},
		{"empty", []string{""}, "must be an integer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newDSMaxTasksSetCmd(&appContext{})
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}
