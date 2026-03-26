package main

import (
	"testing"

	"synocli/internal/apperr"
	"synocli/internal/synology/downloadstation"
	"synocli/internal/synology/filestation"
)

func TestToAppErrorFileStationMessage(t *testing.T) {
	err := toAppError(&filestation.APIError{Code: 408})
	if got := err.Error(); got != "file or folder does not exist" {
		t.Fatalf("unexpected message: %q", got)
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
