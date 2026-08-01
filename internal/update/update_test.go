package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Regression: a self-update download must be bounded before checksum
// verification so a hostile endpoint cannot stream an unbounded body.
func TestDownloadAssetEnforcesSizeCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Stream more than the cap without advertising Content-Length.
		buf := make([]byte, 1024)
		for written := 0; written < 5000; written += len(buf) {
			_, _ = w.Write(buf)
		}
	}))
	defer ts.Close()

	c := NewClient(ts.Client())
	_, err := c.downloadAsset(context.Background(), ts.URL, false, 4096)
	if err == nil {
		t.Fatal("expected size-cap error, got nil")
	}
	if !strings.Contains(err.Error(), "maximum allowed size") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadAssetUnderCap(t *testing.T) {
	payload := []byte("small-payload")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer ts.Close()

	c := NewClient(ts.Client())
	got, err := c.downloadAsset(context.Background(), ts.URL, false, 4096)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

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
		{name: "current dev not clobbered", latest: "v1.2.3", current: "dev", want: false},
		{name: "invalid latest", latest: "latest", current: "v1.2.3", want: false},
		{name: "git-describe suffix equal", latest: "v1.2.3", current: "v1.2.3-5-gabc1234", want: false},
		{name: "git-describe suffix older", latest: "v1.2.4", current: "v1.2.3-5-gabc1234", want: true},
		{name: "prerelease suffix", latest: "v1.2.3", current: "v1.2.3-rc1", want: false},
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
