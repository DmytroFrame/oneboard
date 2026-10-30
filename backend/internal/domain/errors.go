// Package domain holds the business entities and errors. It has no dependencies on
// frameworks, databases or transport — every other layer depends on it, never the reverse.
package domain

import "errors"

// Kind classifies an error so outer layers (HTTP) can map it without knowing the use case.
type Kind int

const (
	KindInternal Kind = iota
	KindValidation
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindTooLarge
)

// Error is a business error. Code is a stable machine-readable identifier that clients
// translate (e.g. "project_not_found"); Msg is an English fallback.
type Error struct {
	Kind Kind
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func Validation(code, msg string) error   { return &Error{KindValidation, code, msg} }
func Unauthorized(code, msg string) error { return &Error{KindUnauthorized, code, msg} }
func Forbidden(code, msg string) error    { return &Error{KindForbidden, code, msg} }
func NotFound(code, msg string) error     { return &Error{KindNotFound, code, msg} }
func Conflict(code, msg string) error     { return &Error{KindConflict, code, msg} }
func TooLarge(code, msg string) error     { return &Error{KindTooLarge, code, msg} }

// ErrNoRows is returned by repositories when a single-row lookup finds nothing.
// Use cases translate it into a NotFound with a meaningful code.
var ErrNoRows = errors.New("not found")

// ErrDuplicate is returned by repositories on a unique constraint violation.
var ErrDuplicate = errors.New("duplicate")

// AsError returns the business error inside err, if any.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
