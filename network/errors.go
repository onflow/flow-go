package network

import (
	"errors"
	"fmt"
)

var (
	EmptyTargetList = errors.New("target list empty")
)

// TransientError represents an error returned from a network layer function call
// which may be interpreted as non-critical. In general, we desire that all expected
// error return values are enumerated in a function's documentation - any undocumented
// errors are considered fatal. However, 3rd party libraries don't always conform to
// this standard, including the networking libraries we use. This error type can be
// used to wrap these 3rd party errors on the boundary into flow-go, to explicitly
// mark them as non-critical.
type TransientError struct {
	Err error
}

func (err TransientError) Error() string {
	return err.Err.Error()
}

func (err TransientError) Unwrap() error {
	return err.Err
}

func NewTransientErrorf(msg string, args ...interface{}) TransientError {
	return TransientError{
		Err: fmt.Errorf(msg, args...),
	}
}

func IsTransientError(err error) bool {
	var errClusterVotingFailed TransientError
	return errors.As(err, &errClusterVotingFailed)
}
