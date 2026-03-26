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
