package handler

import (
	"errors"
	"fmt"
)

// BackendError is a non-fatal error wraps errors returned from the backend
type BackendError struct {
	err error
}

// NewBackendError returns a new BackendError
func NewBackendError(rootCause error) BackendError {
	return BackendError{
		err: rootCause,
	}
}

// Unwrap unwraps the underlying evm error
func (err BackendError) Unwrap() error {
	return err.err
}

func (err BackendError) Error() string {
	return fmt.Sprintf("backend error: %v", err.err)
}

// IsABackendError returns true if the error or any underlying errors
// is a backend error
func IsABackendError(err error) bool {
	return errors.As(err, &BackendError{})
}
