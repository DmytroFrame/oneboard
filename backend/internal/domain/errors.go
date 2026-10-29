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

// Error is a business error with a message safe to show to the user.
type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func Validation(msg string) error   { return &Error{KindValidation, msg} }
func Unauthorized(msg string) error { return &Error{KindUnauthorized, msg} }
func Forbidden(msg string) error    { return &Error{KindForbidden, msg} }
func NotFound(msg string) error     { return &Error{KindNotFound, msg} }
func Conflict(msg string) error     { return &Error{KindConflict, msg} }
func TooLarge(msg string) error     { return &Error{KindTooLarge, msg} }

// ErrNoRows is returned by repositories when a single-row lookup finds nothing.
// Use cases translate it into a NotFound with a meaningful message.
var ErrNoRows = errors.New("not found")

// ErrDuplicate is returned by repositories on a unique constraint violation.
var ErrDuplicate = errors.New("duplicate")

// KindOf returns the error's Kind, or KindInternal for unexpected errors.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}
