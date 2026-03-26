package main

import (
	"testing"

	"synocli/internal/synology/filestation"
)

func TestToAppErrorFileStationMessage(t *testing.T) {
	err := toAppError(&filestation.APIError{Code: 408})
	if got := err.Error(); got != "file or folder does not exist" {
		t.Fatalf("unexpected message: %q", got)
	}
}
