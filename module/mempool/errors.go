package mempool

import (
	"errors"
	"fmt"
)

// UnknownExecutionResultError indicates that the Execution Result is unknown
type UnknownExecutionResultError struct {
	err error
}

func NewUnknownExecutionResultErrorf(msg string, args ...any) error {
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

// BelowPrunedThresholdError indicates that we are attempting to query or prune a mempool by a
// key (typically block height or block view) which is lower than the lowest retained key threshold.
// In other words, we have already pruned above the specified key value.
type BelowPrunedThresholdError struct {
	err error
}

func NewBelowPrunedThresholdErrorf(msg string, args ...any) error {
	return BelowPrunedThresholdError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e BelowPrunedThresholdError) Unwrap() error {
	return e.err
}

func (e BelowPrunedThresholdError) Error() string {
	return e.err.Error()
}

// IsBelowPrunedThresholdError returns whether the given error is an BelowPrunedThresholdError error
func IsBelowPrunedThresholdError(err error) bool {
	var newIsBelowPrunedThresholdError BelowPrunedThresholdError
	return errors.As(err, &newIsBelowPrunedThresholdError)
}
