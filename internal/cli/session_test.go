package cli

import (
	"errors"
	"fmt"
	"testing"

	"synocli/internal/synology/filestation"
)

// Regression: a batch op (e.g. `fs search clear`) must treat only genuine
// per-task FileStation errors as "this id failed"; session-expiry and
// transport errors must propagate so withSession can retry/re-login.
func TestIsPerTaskFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"per-task filestation error", &filestation.APIError{Code: 408}, true},
		{"session expiry", &filestation.APIError{Code: 106}, false},
		{"session expiry 107", &filestation.APIError{Code: 107}, false},
		{"transport error", errors.New("connection refused"), false},
		{"wrapped transport error", fmt.Errorf("call failed: %w", errors.New("eof")), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPerTaskFailure(tc.err); got != tc.want {
				t.Fatalf("isPerTaskFailure(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
