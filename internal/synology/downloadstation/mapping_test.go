package downloadstation

import (
	"testing"
)

func TestMapTaskIncludesStatusEnumFields(t *testing.T) {
	m := MapTask(Task{
		ID:     "dbid_1",
		Title:  "x",
		Status: "3",
	})
	if got := m["raw_status"]; got != "3" {
		t.Fatalf("raw_status=%v want 3", got)
	}
	if got := m["status_enum"]; got != "paused" {
		t.Fatalf("status_enum=%v want paused", got)
	}
	if got := m["status_display"]; got != "paused (3)" {
		t.Fatalf("status_display=%v want paused (3)", got)
	}
	code, ok := m["status_code"].(int)
	if !ok || code != 3 {
		t.Fatalf("status_code=%v want int(3)", m["status_code"])
	}
}

func TestETASecondsOf(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want int64
	}{
		{
			name: "known eta exact division",
			task: Task{
				Size: 1000,
				Additional: &TaskAdditional{
					Transfer: &TaskTransfer{SizeDownloaded: 400, SpeedDownload: 100},
				},
			},
			want: 6,
		},
		{
			name: "known eta ceil division",
			task: Task{
				Size: 1001,
				Additional: &TaskAdditional{
					Transfer: &TaskTransfer{SizeDownloaded: 400, SpeedDownload: 100},
				},
			},
			want: 7,
		},
		{
			name: "completed",
			task: Task{
				Size: 500,
				Additional: &TaskAdditional{
					Transfer: &TaskTransfer{SizeDownloaded: 500, SpeedDownload: 10},
				},
			},
			want: 0,
		},
		{
			name: "unknown when speed is zero",
			task: Task{
				Size: 500,
				Additional: &TaskAdditional{
					Transfer: &TaskTransfer{SizeDownloaded: 100, SpeedDownload: 0},
				},
			},
			want: -1,
		},
		{
			name: "unknown when size is zero",
			task: Task{
				Size: 0,
				Additional: &TaskAdditional{
					Transfer: &TaskTransfer{SizeDownloaded: 10, SpeedDownload: 5},
				},
			},
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ETASecondsOf(tt.task); got != tt.want {
				t.Fatalf("ETASecondsOf()=%d want %d", got, tt.want)
			}
		})
	}
}

func TestMapTaskIncludesETASeconds(t *testing.T) {
	m := MapTask(Task{
		ID:    "dbid_1",
		Title: "x",
		Size:  1000,
		Additional: &TaskAdditional{
			Transfer: &TaskTransfer{SizeDownloaded: 400, SpeedDownload: 100},
		},
	})
	got, ok := m["eta_seconds"].(int64)
	if !ok {
		t.Fatalf("eta_seconds type=%T want int64", m["eta_seconds"])
	}
	if got != 6 {
		t.Fatalf("eta_seconds=%d want 6", got)
	}
}
