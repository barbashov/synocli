package cmdutil

import "fmt"

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
