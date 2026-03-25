package apperr

import "fmt"

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

func Wrap(code, message string, exitCode int, err error) *Error {
	return &Error{Code: code, Message: message, ExitCode: exitCode, Err: err}
}

func New(code, message string, exitCode int) *Error {
	return &Error{Code: code, Message: message, ExitCode: exitCode}
}

func ExitCode(err error) int {
	var app *Error
	if ok := As(err, &app); ok {
		if app.ExitCode > 0 {
			return app.ExitCode
		}
	}
	return 1
}

func Code(err error) string {
	var app *Error
	if ok := As(err, &app); ok && app.Code != "" {
		return app.Code
	}
	return "internal_error"
}

func Details(err error) map[string]any {
	var app *Error
	if ok := As(err, &app); ok {
		return app.Details
	}
	return nil
}

func As(err error, target **Error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		*target = e
		return true
	}
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return As(u.Unwrap(), target)
	}
	return false
}
