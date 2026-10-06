package errs

import (
	"context"
	"errors"
	"fmt"
)

// Error carries a public CLI exit code while retaining the original diagnostic.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }
func New(code int, format string, args ...any) error {
	return &Error{code, fmt.Errorf(format, args...)}
}
func Wrap(code int, err error) error {
	if err == nil {
		return nil
	}
	return &Error{code, err}
}
func Code(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 3
}
