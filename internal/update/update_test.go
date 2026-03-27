package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{name: "newer patch", latest: "v1.2.4", current: "v1.2.3", want: true},
		{name: "equal", latest: "v1.2.3", current: "v1.2.3", want: false},
		{name: "older", latest: "v1.2.2", current: "v1.2.3", want: false},
		{name: "current dev", latest: "v1.2.3", current: "dev", want: true},
		{name: "invalid latest", latest: "latest", current: "v1.2.3", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNewerVersion(tt.latest, tt.current); got != tt.want {
				t.Fatalf("IsNewerVersion(%q, %q)=%t want %t", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func TestShouldBackgroundCheck(t *testing.T) {
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	if !ShouldBackgroundCheck(now, State{}) {
		t.Fatal("expected check on empty state")
	}

	success := State{LastAttemptAt: now.Add(-23 * time.Hour), LastSuccessAt: now.Add(-23 * time.Hour)}
	if ShouldBackgroundCheck(now, success) {
		t.Fatal("unexpected check before successful 24h interval")
	}
	success.LastAttemptAt = now.Add(-24 * time.Hour)
	success.LastSuccessAt = now.Add(-24 * time.Hour)
	if !ShouldBackgroundCheck(now, success) {
		t.Fatal("expected check at successful 24h interval")
	}

	failed := State{LastAttemptAt: now.Add(-5 * time.Hour), LastSuccessAt: now.Add(-26 * time.Hour)}
	if ShouldBackgroundCheck(now, failed) {
		t.Fatal("unexpected check before failure cooldown")
	}
	failed.LastAttemptAt = now.Add(-6 * time.Hour)
	if !ShouldBackgroundCheck(now, failed) {
		t.Fatal("expected check at failure cooldown")
	}
}

func TestStateReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "update-check.json")
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	want := State{LastAttemptAt: now, LastSuccessAt: now, LatestVersion: "v0.3.3"}
	if err := WriteState(p, want); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat state file: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("state mode=%04o want 0600", st.Mode().Perm())
	}
	got, err := LoadState(p)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if !got.LastAttemptAt.Equal(want.LastAttemptAt) || !got.LastSuccessAt.Equal(want.LastSuccessAt) || got.LatestVersion != want.LatestVersion {
		t.Fatalf("unexpected state: %#v", got)
	}
}

func TestAssetName(t *testing.T) {
	name, err := AssetName("v0.3.2", "linux", "amd64")
	if err != nil {
		t.Fatalf("AssetName: %v", err)
	}
	if name != "synocli_v0.3.2_linux_amd64.tar.gz" {
		t.Fatalf("unexpected asset name: %s", name)
	}
	if _, err := AssetName("v0.3.2", "linux", "ppc64"); err == nil {
		t.Fatal("expected unsupported architecture error")
	}
}
