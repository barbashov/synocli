package filestation

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MapSliceAny casts a []any to []map[string]any, dropping non-map elements.
func MapSliceAny(v any) []map[string]any {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// ValueFromMap returns the string representation of m[k], or "" if absent.
func ValueFromMap(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// Int64FromAny converts various numeric types to int64.
func Int64FromAny(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	case json.Number:
		n, err := t.Int64()
		if err == nil {
			return n, true
		}
	case string:
		var n int64
		_, err := fmt.Sscanf(strings.TrimSpace(t), "%d", &n)
		if err == nil {
			return n, true
		}
	}
	return 0, false
}

// FirstTaskID extracts a task ID from a response map, trying multiple key names.
func FirstTaskID(m map[string]any) string {
	for _, key := range []string{"taskid", "task_id", "taskId"} {
		if v, ok := m[key]; ok {
			s := firstString(v)
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func firstString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	case []string:
		for _, s := range t {
			if s != "" {
				return s
			}
		}
	}
	return ""
}
