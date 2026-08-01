package cmdutil

import (
	"fmt"
	"strings"
)

func FormatBytes(b int64) string {
	// Sizes/speeds from the DSM API are non-negative; clamp anything negative
	// rather than emit a nonsensical unit-less value.
	if b <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	v := float64(b)
	last := len(units) - 1
	for i, u := range units {
		// 1023.95 rounds to "1024.0" under %.1f, so roll such values over to
		// the next unit instead of printing an out-of-range "1024.0 KB".
		if v < 1023.95 || i == last {
			if u == "B" {
				return fmt.Sprintf("%d B", b)
			}
			return fmt.Sprintf("%.1f %s", v, u)
		}
		v /= 1024
	}
	// Unreachable: the loop always returns at the last unit.
	return fmt.Sprintf("%.1f %s", v, units[last])
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
