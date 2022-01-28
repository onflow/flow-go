package crypto

import (
	"errors"
	"fmt"
)

// duplicatedSignerError is an error returned when TrustedAdd or VerifyAndAdd encounter
// a signature share that has been already added to the internal state.
type duplicatedSignerError struct {
	error
}

// duplicatedSignerErrorf constructs a new duplicatedSignerError
func duplicatedSignerErrorf(msg string, args ...interface{}) error {
	return &duplicatedSignerError{error: fmt.Errorf(msg, args...)}
}

// IsDuplicatedSignerError checks if the input error is a duplicatedSignerError
func IsDuplicatedSignerError(err error) bool {
	var target *duplicatedSignerError
	return errors.As(err, &target)
}

// notEnoughSharesError is an error returned when ThresholdSignature is called
// and not enough shares have been collected.
type notEnoughSharesError struct {
	error
}

// notEnoughSharesErrorf constructs a new notEnoughSharesError
func notEnoughSharesErrorf(msg string, args ...interface{}) error {
	return &notEnoughSharesError{error: fmt.Errorf(msg, args...)}
}

// IsNotEnoughSharesError checks if the input error is a notEnoughSharesError
func IsNotEnoughSharesError(err error) bool {
	var target *notEnoughSharesError
	return errors.As(err, &target)
}
