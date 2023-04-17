package derived

import (
	"fmt"
)

type RetryableError interface {
	error
	IsRetryable() bool
}

type retryableError struct {
	error

	isRetryable bool
}

func newRetryableError(msg string, vals ...interface{}) RetryableError {
	return retryableError{
		error:       fmt.Errorf(msg, vals...),
		isRetryable: true,
	}
}

func newNotRetryableError(msg string, vals ...interface{}) RetryableError {
	return retryableError{
		error:       fmt.Errorf(msg, vals...),
		isRetryable: false,
	}
}

func (err retryableError) IsRetryable() bool {
	return err.isRetryable
}
