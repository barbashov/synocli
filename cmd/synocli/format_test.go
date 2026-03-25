package main

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.in); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B/s"},
		{1024, "1.0 KB/s"},
		{5242880, "5.0 MB/s"},
	}
	for _, tt := range tests {
		if got := formatSpeed(tt.in); got != tt.want {
			t.Errorf("formatSpeed(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		downloaded, total int64
		want              string
	}{
		{0, 0, "-"},
		{0, 100, "0.0%"},
		{50, 100, "50.0%"},
		{100, 100, "100.0%"},
		{150, 100, "100.0%"},
		{0, -1, "-"},
	}
	for _, tt := range tests {
		if got := formatPercent(tt.downloaded, tt.total); got != tt.want {
			t.Errorf("formatPercent(%d, %d) = %q, want %q", tt.downloaded, tt.total, got, tt.want)
		}
	}
}
