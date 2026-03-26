package downloadstation

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeCreatePropagatesDetailedAPIError(t *testing.T) {
	_, _, err := decodeCreate(strings.NewReader(`{
		"success": false,
		"error": {
			"code": 400,
			"errors": {"name": "bad_name", "reason": "bad_reason"}
		}
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != 400 || apiErr.Name != "bad_name" || apiErr.Reason != "bad_reason" {
		t.Fatalf("unexpected API error: %#v", apiErr)
	}
}

func TestDecodeBasePropagatesFailedTaskDetails(t *testing.T) {
	err := decodeBase(strings.NewReader(`{
		"success": false,
		"error": {
			"code": 405,
			"errors": {
				"failed_task": [{"id": "dbid_1", "error": 405}]
			}
		}
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != 405 {
		t.Fatalf("unexpected code: %d", apiErr.Code)
	}
	if len(apiErr.FailedTasks) != 1 {
		t.Fatalf("expected 1 failed task, got %#v", apiErr.FailedTasks)
	}
	if apiErr.FailedTasks[0].ID != "dbid_1" || apiErr.FailedTasks[0].Code != 405 {
		t.Fatalf("unexpected failed task: %#v", apiErr.FailedTasks[0])
	}
}

func TestDecodeActionTreatsNonZeroFailedTaskAsError(t *testing.T) {
	err := decodeAction(strings.NewReader(`{
		"success": true,
		"data": {
			"failed_task": [{"id":"dbid_1","error":405}]
		}
	}`))
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != 405 {
		t.Fatalf("unexpected code: %d", apiErr.Code)
	}
	if len(apiErr.FailedTasks) != 1 || apiErr.FailedTasks[0].ID != "dbid_1" || apiErr.FailedTasks[0].Code != 405 {
		t.Fatalf("unexpected failed tasks: %#v", apiErr.FailedTasks)
	}
}

func TestDecodeActionIgnoresZeroFailedTaskCode(t *testing.T) {
	if err := decodeAction(strings.NewReader(`{
		"success": true,
		"data": {
			"failed_task": [{"id":"dbid_1","error":0}]
		}
	}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStringSliceFromAny(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "string", in: "a", want: []string{"a"}},
		{name: "empty string", in: "", want: nil},
		{name: "any slice", in: []any{"a", "", 12, "b"}, want: []string{"a", "b"}},
		{name: "string slice", in: []string{"a", "", "b"}, want: []string{"a", "b"}},
		{name: "unsupported", in: 42, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringSliceFromAny(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got=%#v want=%#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("value mismatch: got=%#v want=%#v", got, tc.want)
				}
			}
		})
	}
}
