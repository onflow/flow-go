package merkle

import (
	"errors"
	"fmt"
)

var ErrorIncompatibleKeyLength = errors.New("key has incompatible size")

// MalformedProofError is returned when a proof format
// has some issues (syntax checks).
type MalformedProofError struct {
	err error
}

// NewMalformedProofErrorf constructs a new MalformedProofError
func NewMalformedProofErrorf(msg string, args ...interface{}) *MalformedProofError {
	return &MalformedProofError{err: fmt.Errorf(msg, args...)}
}

func (e MalformedProofError) Error() string {
	return fmt.Sprintf("malformed proof, %s", e.err.Error())
}

// Unwrap unwraps the error
func (e MalformedProofError) Unwrap() error {
	return e.err
}

// IsMalformedProofError checks if err is a MalformedProofError
func IsMalformedProofError(err error) bool {
	var target *MalformedProofError
	return errors.As(err, &target)
}

// InvalidProofError is returned when proof
// verification has failed (semantic check).
// The most common case for this error is when the computed root hash
// doesn't match the root hash provided to the Verify method.
type InvalidProofError struct {
	err error
}

// newInvalidProofErrorf constructs a new InvalidProofError
func newInvalidProofErrorf(msg string, args ...interface{}) *InvalidProofError {
	return &InvalidProofError{err: fmt.Errorf(msg, args...)}
}

func (e InvalidProofError) Error() string {
	return fmt.Sprintf("invalid proof, %s", e.err.Error())
}

// Unwrap unwraps the error
func (e InvalidProofError) Unwrap() error {
	return e.err
}

// IsInvalidProofError checks if err is a InvalidProofError
func IsInvalidProofError(err error) bool {
	var target *InvalidProofError
	return errors.As(err, &target)
}
