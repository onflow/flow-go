package mempool

import (
	"errors"
	"fmt"
)

// UnknownExecutionResultError indicates that the Execution Result is unknown
type UnknownExecutionResultError struct {
	err error
}

func NewUnknownExecutionResultError(msg string) error {
	return NewUnknownExecutionResultErrorf(msg)
}

func NewUnknownExecutionResultErrorf(msg string, args ...interface{}) error {
	return UnknownExecutionResultError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e UnknownExecutionResultError) Unwrap() error {
	return e.err
}

func (e UnknownExecutionResultError) Error() string {
	return e.err.Error()
}

// IsUnknownExecutionResultError returns whether the given error is an UnknownExecutionResultError error
func IsUnknownExecutionResultError(err error) bool {
	var unknownExecutionResultError UnknownExecutionResultError
	return errors.As(err, &unknownExecutionResultError)
}

// DecreasingPruningHeightError indicates that we are pruning a mempool by a height that
// is lower than existing height
type DecreasingPruningHeightError struct {
	err error
}

func NewDecreasingPruningHeightError(msg string) error {
	return NewDecreasingPruningHeightErrorf(msg)
}

func NewDecreasingPruningHeightErrorf(msg string, args ...interface{}) error {
	return DecreasingPruningHeightError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e DecreasingPruningHeightError) Unwrap() error {
	return e.err
}

func (e DecreasingPruningHeightError) Error() string {
	return e.err.Error()
}

// IsDecreasingPruningHeightError returns whether the given error is an DecreasingPruningHeightError error
func IsDecreasingPruningHeightError(err error) bool {
	var newIsDecreasingPruningHeightError DecreasingPruningHeightError
	return errors.As(err, &newIsDecreasingPruningHeightError)
}
