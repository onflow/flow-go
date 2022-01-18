package merkle

import (
	"fmt"
)

// MalformedProofError is returned when the proof format has an issue
type MalformedProofError struct {
	err error
}

// NewErrorMalformedProoff constructs a new MalformedProofError
func NewErrorMalformedProoff(msg string, args ...interface{}) *MalformedProofError {
	return &MalformedProofError{err: fmt.Errorf(msg, args...)}
}

func (e MalformedProofError) Error() string {
	return fmt.Sprintf("malformed proof, %s", e.err.Error())
}

// Unwrap unwraps the error
func (e MalformedProofError) Unwrap() error {
	return e.err
}
