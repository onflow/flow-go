package irrecoverable

import (
	"fmt"
)

// exception represents an unexpected error. An unexpected error is any error returned
// by a function, other than the error specifically documented as expected in that
// function's interface.
//
// It wraps an error, which could be a sentinel error. IT does NOT IMPLEMENT an UNWRAP method,
// so the enclosed error's type cannot be accessed. Therefore, methods such as `errors.As` and
// `errors.Is` do not detect the exception as any known sentinel error.
// NOTE: this type is private, because checking for an exception (as opposed to checking for the
// set of expected error types) is considered an anti-pattern because it may miss unexpected
// errors which are not implemented as an exception.
//
// Functions may return an exception when:
//   - they are interpreting any error returning from a 3rd party module as unexpected
//   - they are reacting to an unexpected condition internal to their stack frame and returning a generic error
//
// Functions must return an exception when:
//   - they are interpreting any documented sentinel error returned from a flow-go module as unexpected
type exception struct {
	err error
}

// Error returns the error string of the exception. It is always prefixed by `[exception!]`
// to easily differentiate unexpected errors in logs.
func (e exception) Error() string {
	return "[exception!] " + e.err.Error()
}

// NewException wraps the input error as an exception, stripping any sentinel error information.
// This ensures that all upper levels in the stack will consider this an unexpected error.
func NewException(err error) error {
	return exception{err: err}
}

// NewExceptionf is NewException with the ability to add formatting and context to the error text.
func NewExceptionf(msg string, args ...any) error {
	return NewException(fmt.Errorf(msg, args...))
}
