package p2p

import (
	"errors"
	"fmt"
)

// MiddlewareStartError covers the entire class of errors returned from Middleware.start .
// This error is returned if the Middleware fails to start for any reason.
// The networking layer will not be functional if this error condition is encountered.
type MiddlewareStartError struct {
	err error
}

func NewMiddlewareStartErrorf(msg string, args ...interface{}) error {
	return MiddlewareStartError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e MiddlewareStartError) Error() string { return e.err.Error() }
func (e MiddlewareStartError) Unwrap() error { return e.err }

// IsMiddlewareStartError returns whether err is an MiddlewareStartError
func IsMiddlewareStartError(err error) bool {
	var e MiddlewareStartError
	return errors.As(err, &e)
}
