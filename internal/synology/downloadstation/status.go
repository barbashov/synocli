package downloadstation

import (
	"fmt"
	"strconv"
	"strings"
)

func NormalizeStatus(raw string) string {
	if code, ok := parseNumericStatusCode(raw); ok {
		switch code {
		case 1:
			return "waiting"
		case 2, 6:
			return "downloading"
		case 3:
			return "paused"
		case 4, 8, 9:
			return "finishing"
		case 5:
			return "finished"
		case 7:
			return "seeding"
		case 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35:
			return "error"
		case 36:
			return "unknown"
		default:
			return "unknown"
		}
	}
	s := strings.ToLower(raw)
	switch s {
	case "waiting", "waiting_peer", "waiting_tracker":
		return "waiting"
	case "downloading", "hash_checking":
		return "downloading"
	case "paused":
		return "paused"
	case "finishing", "extracting", "filehosting_waiting":
		return "finishing"
	case "finished":
		return "finished"
	case "seeding":
		return "seeding"
	case "error":
		return "error"
	default:
		return "unknown"
	}
}

var ds2StatusEnum = map[int]string{
	1:  "waiting",
	2:  "downloading",
	3:  "paused",
	4:  "finishing",
	5:  "finished",
	6:  "hashing",
	7:  "seeding",
	8:  "filehosting_waiting",
	9:  "extracting",
	10: "error",
	11: "broken_link",
	12: "destination_not_exist",
	13: "destination_denied",
	14: "disk_full",
	15: "quota_reached",
	16: "timeout",
	17: "exceed_max_file_system_size",
	18: "exceed_max_destination_size",
	19: "exceed_max_temp_size",
	20: "encrypted_name_too_long",
	21: "name_too_long",
	22: "torrent_duplicate",
	23: "file_not_exist",
	24: "required_premium_account",
	25: "not_supported_type",
	26: "try_it_later",
	27: "task_encryption",
	28: "missing_python",
	29: "private_video",
	30: "ftp_encryption_not_supported_type",
	31: "extract_failed",
	32: "extract_failed_wrong_password",
	33: "extract_failed_invalid_archive",
	34: "extract_failed_quota_reached",
	35: "extract_failed_disk_full",
	36: "unknown",
}

func StatusEnum(raw string) string {
	if code, ok := parseNumericStatusCode(raw); ok {
		if name, ok := ds2StatusEnum[code]; ok {
			return name
		}
		return "unknown"
	}
	return strings.ToLower(raw)
}

func StatusCode(raw string) (int, bool) {
	return parseNumericStatusCode(raw)
}

func StatusDisplay(raw string) string {
	if code, ok := parseNumericStatusCode(raw); ok {
		if name, ok := ds2StatusEnum[code]; ok {
			return fmt.Sprintf("%s (%d)", name, code)
		}
		return fmt.Sprintf("unknown (%d)", code)
	}
	return raw
}

func parseNumericStatusCode(raw string) (int, bool) {
	code, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}
	return code, true
}

func IsTerminalSuccess(normalized string) bool {
	return normalized == "finished" || normalized == "seeding"
}

func IsTerminalFailure(normalized string) bool {
	return normalized == "error"
}
