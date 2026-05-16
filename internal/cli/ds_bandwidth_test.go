package cli

import (
	"strings"
	"testing"
)

func TestDSBandwidthHasGetAndSet(t *testing.T) {
	cmd := newDSBandwidthCmd(&appContext{})
	for _, name := range []string{"get", "set"} {
		sub, _, err := cmd.Find([]string{name})
		if err != nil || sub == nil || sub.Name() != name {
			t.Fatalf("bandwidth %s not found: sub=%#v err=%v", name, sub, err)
		}
	}
}

func TestDSBandwidthSetRequiresAtLeastOneFlag(t *testing.T) {
	cmd := newDSBandwidthSetCmd(&appContext{})
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error when no flags provided")
	}
	if !strings.Contains(err.Error(), "at least one of --bt-max-download or --bt-max-upload") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDSBandwidthSetRejectsNegativeValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"negative-download", []string{"--bt-max-download", "-1"}, "--bt-max-download must be >= 0"},
		{"negative-upload", []string{"--bt-max-upload", "-5"}, "--bt-max-upload must be >= 0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newDSBandwidthSetCmd(&appContext{})
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

func TestFormatSpeedLimit(t *testing.T) {
	if got := formatSpeedLimit(0); got != "unlimited" {
		t.Errorf("formatSpeedLimit(0)=%q want unlimited", got)
	}
	if got := formatSpeedLimit(1024); got != "1024 KB/s" {
		t.Errorf("formatSpeedLimit(1024)=%q want 1024 KB/s", got)
	}
}
