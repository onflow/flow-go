package errors

// UnreachableError

// UnreachableError is an internal error in the runtime which should have never occurred
// due to a programming error in the runtime.
//
// NOTE: this error is not used for errors because of bugs in a user-provided program.
// For program errors, see execution/strictus/interpreter/errors.go
//
type UnreachableError struct{}

func (UnreachableError) Error() string {
	return "unreachable"
}

// SecondaryError

// SecondaryError is an interface for errors that provide a secondary error message
//
type SecondaryError interface {
	SecondaryError() string
}
