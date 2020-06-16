package engine

import (
	"errors"
	"fmt"
)

// InvalidInputError are the type for input errors. It's useful to distinguish
// errors from exceptions.
// By distinguishing errors from exception using different type, we can log them
// differently.
// For instance, log InvalidInputError error as a warning log, and log
// other error as an error log.
// You can use this struct as an customized error type directly or
// create a function to reuse a certain error type, just like ErrorExecutionResultExist
type InvalidInputError struct {
	err error
}

func NewInvalidInputError(msg string) error {
	return NewInvalidInputErrorf(msg)
}

func NewInvalidInputErrorf(msg string, args ...interface{}) error {
	return InvalidInputError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InvalidInputError) Unwrap() error {
	return e.err
}

func (e InvalidInputError) Error() string {
	return e.err.Error()
}

// IsInvalidInputError returns whether the given error is an InvalidInputError error
func IsInvalidInputError(err error) bool {
	var errInvalidInputError InvalidInputError
	return errors.As(err, &errInvalidInputError)
}
