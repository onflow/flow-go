package signature

import (
	"errors"
	"fmt"
)

var (
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")
)

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

// ErrInvalidFormat is returned an invalid format or signature is detected
type ErrInvalidFormat struct {
	message string
}

// NewErrInvalidFormat constructs a new ErrInvalidFormat
func NewErrInvalidFormat(msg string, args ...interface{}) error {
	return &ErrInvalidFormat{message: fmt.Sprintf(msg, args...)}
}

func (e ErrInvalidFormat) Error() string {
	return e.message
}
