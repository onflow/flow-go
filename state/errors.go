package state

import (
	"errors"
	"fmt"
)

// InvalidExtensionError is an error for invalid extension of the state
type InvalidExtensionError struct {
	msg string
}

func NewInvalidExtensionError(msg string) error {
	return InvalidExtensionError{
		msg: msg,
	}
}

func NewInvalidExtensionErrorf(msg string, args ...interface{}) error {
	return NewInvalidExtensionError(fmt.Sprintf(msg, args...))
}

func (e InvalidExtensionError) Error() string {
	return e.msg
}

// IsInvalidExtensionError returns whether the given error is an InvalidExtensionError error
func IsInvalidExtensionError(err error) bool {
	var errInvalidExtensionError InvalidExtensionError
	return errors.As(err, &errInvalidExtensionError)
}

// OutdatedExtensionError is an error for the extension of the state being outdated.
// Being outdated doesn't mean it's invalid or not.
// Knowing whether an outdated extension is an invalid extension or not would
// take more state queries.
type OutdatedExtensionError struct {
	msg string
}

func NewOutdatedExtensionError(msg string) error {
	return OutdatedExtensionError{
		msg: msg,
	}
}

func NewOutdatedExtensionErrorf(msg string, args ...interface{}) error {
	return NewOutdatedExtensionError(fmt.Sprintf(msg, args...))
}

func (e OutdatedExtensionError) Error() string {
	return e.msg
}

func IsOutdatedExtensionError(err error) bool {
	var errOutdatedExtensionError OutdatedExtensionError
	return errors.As(err, &errOutdatedExtensionError)
}
