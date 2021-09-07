package signature

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidFormat      = errors.New("invalid signature format")
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")
)

// ErrInvalidInputs is returned when an API receives invalid inputs.
type ErrInvalidInputs struct {
	message string
}

// newErrInvalidInputs constructs a new ErrInvalidInputs
func newErrInvalidInputs(msg string, args ...interface{}) error {
	return &ErrInvalidInputs{message: fmt.Sprintf(msg, args...)}
}

func (e ErrInvalidInputs) Error() string {
	return e.message
}

// ErrDuplicatedSigner is returned when an API detects a duplicate signer when it shouldn't
type ErrDuplicatedSigner struct {
	message string
}

// newErrDuplicatedSigner constructs a new ErrDuplicatedSigner
func newErrDuplicatedSigner(msg string, args ...interface{}) error {
	return &ErrDuplicatedSigner{message: fmt.Sprintf(msg, args...)}
}

func (e ErrDuplicatedSigner) Error() string {
	return e.message
}
