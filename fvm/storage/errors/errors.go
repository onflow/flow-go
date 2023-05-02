package errors

import (
	stdErrors "errors"
	"fmt"
)

type Unwrappable interface {
	Unwrap() error
}

type RetryableConflictError interface {
	IsRetryableConflict() bool

	Unwrappable
	error
}

func IsRetryableConflictError(originalErr error) bool {
	if originalErr == nil {
		return false
	}

	currentErr := originalErr
	for {
		var retryable RetryableConflictError
		if !stdErrors.As(currentErr, &retryable) {
			return false
		}

		if retryable.IsRetryableConflict() {
			return true
		}

		currentErr = retryable.Unwrap()
	}
}

type retryableConflictError struct {
	error
}

func NewRetryableConflictError(
	msg string,
	vals ...interface{},
) error {
	return &retryableConflictError{
		error: fmt.Errorf(msg, vals...),
	}
}

func (retryableConflictError) IsRetryableConflict() bool {
	return true
}

func (err *retryableConflictError) Unwrap() error {
	return err.error
}
