package cli

import (
	"errors"
	"testing"

	"synocli/internal/apperr"
	"synocli/internal/synology/downloadstation"
	"synocli/internal/synology/filestation"
)

func TestToAppErrorFileStationMessage(t *testing.T) {
	err := toAppError(&filestation.APIError{Code: 408})
	// The mapped human message leads; the wrapped APIError detail follows
	// (consistent with the Download Station branch, which also wraps Err).
	if got := err.Error(); got != "file or folder does not exist: file station api error code=408 (file or folder does not exist)" {
		t.Fatalf("unexpected message: %q", got)
	}
}

// Regression: the FileStation branch must preserve the underlying error so the
// original *filestation.APIError stays unwrappable (parity with the DS branch).
func TestToAppErrorFileStationPreservesWrappedError(t *testing.T) {
	orig := &filestation.APIError{Code: 408}
	err := toAppError(orig)
	var fsErr *filestation.APIError
	if !errors.As(err, &fsErr) {
		t.Fatalf("underlying *filestation.APIError not unwrappable from %T", err)
	}
	if fsErr != orig {
		t.Fatalf("unwrapped error is not the original")
	}
}

func TestToAppErrorDownloadStationTaskNotFoundExitCode(t *testing.T) {
	// DS v1 returns 404 and DS2 returns 501 for "task does not exist"; both
	// must surface as exit code 3 so callers can branch on it regardless of
	// NAS firmware.
	for _, code := range []int{404, 501} {
		err := toAppError(&downloadstation.APIError{Code: code})
		if got := apperr.ExitCode(err); got != 3 {
			t.Fatalf("ExitCode for synology code %d = %d, want 3", code, got)
		}
	}
}

func TestToAppErrorDownloadStationFailedTaskDetails(t *testing.T) {
	err := toAppError(&downloadstation.APIError{
		Code: 405,
		FailedTasks: []downloadstation.FailedTask{
			{ID: "dbid_1", Code: 405},
		},
	})
	details := apperr.Details(err)
	if details["synology_code"] != 405 {
		t.Fatalf("unexpected synology_code: %#v", details["synology_code"])
	}
	failed, ok := details["failed_tasks"].([]map[string]any)
	if !ok || len(failed) != 1 {
		t.Fatalf("unexpected failed_tasks payload: %#v", details["failed_tasks"])
	}
	if failed[0]["id"] != "dbid_1" || failed[0]["code"] != 405 {
		t.Fatalf("unexpected failed_tasks entry: %#v", failed[0])
	}
}
