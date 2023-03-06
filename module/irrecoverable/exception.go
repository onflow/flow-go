package irrecoverable

import (
	"fmt"
)

// exception represents an unexpected error. It wraps an error, which could be a sentinelVar error.
// IT does NOT IMPLEMENT an UNWRAP method, so the enclosed error's type cannot be accessed.
// Therefore, methods such as `errors.As` and `errors.Is` do not detect the exception as a known sentinelVar error.
type exception struct {
	err error
}

// Error returns the error string of the exception. It is always prefixed by `[exception!]`
// to easily differentiate unexpected errors in logs.
func (e exception) Error() string {
	return "[exception!] " + e.err.Error()
}

// NewException wraps the input error as an exception, stripping any sentinelVar error information.
// This ensures that all upper levels in the stack will consider this an unexpected error.
func NewException(err error) error {
	return exception{err: err}
}

// NewExceptionf is NewException with the ability to add formatting and context to the error text.
func NewExceptionf(msg string, args ...any) error {
	return NewException(fmt.Errorf(msg, args...))
}
