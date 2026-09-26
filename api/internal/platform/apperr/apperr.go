// Package apperr defines the errors domain packages return. No domain package
// knows about HTTP; cmd/api maps these to status codes in exactly one place.
package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
	ErrValidation = errors.New("validation failed")
)

type kindError struct {
	kind error
	msg  string
}

func (e *kindError) Error() string { return e.msg }
func (e *kindError) Unwrap() error { return e.kind }

func NotFound(format string, args ...any) error {
	return &kindError{kind: ErrNotFound, msg: fmt.Sprintf(format, args...)}
}

func Forbidden(format string, args ...any) error {
	return &kindError{kind: ErrForbidden, msg: fmt.Sprintf(format, args...)}
}

func Conflict(format string, args ...any) error {
	return &kindError{kind: ErrConflict, msg: fmt.Sprintf(format, args...)}
}

// ValidationError carries the offending field so the handler can report it in
// the problem+json errors[] array with a location the UI can map to an input.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Msg) }
func (e *ValidationError) Unwrap() error { return ErrValidation }

func Invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Msg: fmt.Sprintf(format, args...)}
}
