package apperr

import (
	"errors"
	"fmt"
)

type Error struct {
	Code     string
	Message  string
	ExitCode int
	Details  map[string]any
	Err      error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func Wrap(code, message string, exitCode int, err error) *Error {
	return &Error{Code: code, Message: message, ExitCode: exitCode, Err: err}
}

func New(code, message string, exitCode int) *Error {
	return &Error{Code: code, Message: message, ExitCode: exitCode}
}

func ExitCode(err error) int {
	var app *Error
	if errors.As(err, &app) {
		if app.ExitCode > 0 {
			return app.ExitCode
		}
	}
	return 1
}

func Code(err error) string {
	var app *Error
	if errors.As(err, &app) && app.Code != "" {
		return app.Code
	}
	return "internal_error"
}

func Details(err error) map[string]any {
	var app *Error
	if errors.As(err, &app) {
		return app.Details
	}
	return nil
}
