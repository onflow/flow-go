package module

import (
	"errors"
	"fmt"
)

// UnknownBlockError indicates that a referenced block is missing
type UnknownBlockError struct {
	err error
}

func NewUnknownBlockError(msg string, args ...interface{}) error {
	return UnknownBlockError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e UnknownBlockError) Unwrap() error {
	return e.err
}

func (e UnknownBlockError) Error() string {
	return e.err.Error()
}

func IsUnknownBlockError(err error) bool {
	var unknownExecutedBlockError UnknownBlockError
	return errors.As(err, &unknownExecutedBlockError)
}

// UnknownResultError indicates that a referenced result is missing
type UnknownResultError struct {
	err error
}

func NewUnknownResultError(msg string, args ...interface{}) error {
	return UnknownResultError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e UnknownResultError) Unwrap() error {
	return e.err
}

func (e UnknownResultError) Error() string {
	return e.err.Error()
}

func IsUnknownResultError(err error) bool {
	var unknownParentResultError UnknownResultError
	return errors.As(err, &unknownParentResultError)
}
