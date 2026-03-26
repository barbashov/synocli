package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorMessage(t *testing.T) {
	e := &Error{Code: "test", Message: "something broke", ExitCode: 2}
	if got := e.Error(); got != "something broke" {
		t.Fatalf("Error()=%q want %q", got, "something broke")
	}
}

func TestErrorMessageWithWrapped(t *testing.T) {
	inner := fmt.Errorf("root cause")
	e := Wrap("test", "outer", 1, inner)
	if got := e.Error(); got != "outer: root cause" {
		t.Fatalf("Error()=%q want %q", got, "outer: root cause")
	}
}

func TestErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("inner")
	e := Wrap("test", "outer", 1, inner)
	if !errors.Is(e, inner) {
		t.Fatal("Unwrap should return inner error")
	}
}

func TestNilErrorMessage(t *testing.T) {
	var e *Error
	if got := e.Error(); got != "" {
		t.Fatalf("nil Error()=%q want empty", got)
	}
}

func TestExitCode(t *testing.T) {
	if got := ExitCode(New("x", "m", 3)); got != 3 {
		t.Fatalf("ExitCode=%d want 3", got)
	}
	if got := ExitCode(fmt.Errorf("plain")); got != 1 {
		t.Fatalf("ExitCode=%d want 1 for plain error", got)
	}
	if got := ExitCode(&Error{Code: "x", Message: "m", ExitCode: 0}); got != 1 {
		t.Fatalf("ExitCode=%d want 1 for zero exit code", got)
	}
}

func TestCode(t *testing.T) {
	if got := Code(New("my_code", "m", 1)); got != "my_code" {
		t.Fatalf("Code=%q want my_code", got)
	}
	if got := Code(fmt.Errorf("plain")); got != "internal_error" {
		t.Fatalf("Code=%q want internal_error for plain error", got)
	}
}

func TestDetails(t *testing.T) {
	e := &Error{Code: "x", Message: "m", ExitCode: 1, Details: map[string]any{"key": "val"}}
	d := Details(e)
	if d["key"] != "val" {
		t.Fatalf("Details=%v want key=val", d)
	}
	if got := Details(fmt.Errorf("plain")); got != nil {
		t.Fatalf("Details=%v want nil for plain error", got)
	}
}

func TestExitCodeWrapped(t *testing.T) {
	inner := New("inner", "m", 7)
	outer := fmt.Errorf("wrapping: %w", inner)
	if got := ExitCode(outer); got != 7 {
		t.Fatalf("ExitCode=%d want 7 through wrapping", got)
	}
}
