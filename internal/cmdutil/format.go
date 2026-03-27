package cmdutil

import (
	"fmt"
	"strings"
)

func FormatBytes(b int64) string {
	if b == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(b)
	for _, u := range units {
		if v < 1024 || u == "TB" {
			if u == "B" {
				return fmt.Sprintf("%d B", b)
			}
			return fmt.Sprintf("%.1f %s", v, u)
		}
		v /= 1024
	}
	return fmt.Sprintf("%.1f TB", v) // unreachable: loop handles TB
}

func FormatSpeed(bps int64) string {
	return FormatBytes(bps) + "/s"
}

func FormatPercent(downloaded, total int64) string {
	if total <= 0 {
		return "-"
	}
	pct := float64(downloaded) / float64(total) * 100.0
	if pct > 100.0 {
		pct = 100.0
	}
	return fmt.Sprintf("%.1f%%", pct)
}

func FormatDurationWords(seconds int64) string {
	if seconds <= 0 {
		return "0 seconds"
	}
	type unit struct {
		seconds  int64
		singular string
		plural   string
	}
	units := []unit{
		{seconds: 24 * 60 * 60, singular: "day", plural: "days"},
		{seconds: 60 * 60, singular: "hour", plural: "hours"},
		{seconds: 60, singular: "minute", plural: "minutes"},
		{seconds: 1, singular: "second", plural: "seconds"},
	}
	parts := make([]string, 0, 2)
	remaining := seconds
	for _, u := range units {
		if len(parts) == 2 {
			break
		}
		count := remaining / u.seconds
		if count == 0 {
			continue
		}
		remaining %= u.seconds
		label := u.plural
		if count == 1 {
			label = u.singular
		}
		parts = append(parts, fmt.Sprintf("%d %s", count, label))
	}
	if len(parts) == 0 {
		return "0 seconds"
	}
	return strings.Join(parts, " ")
}
