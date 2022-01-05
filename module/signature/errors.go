package signature

import (
	"errors"
	"fmt"
)

var (
	ErrInsufficientShares = errors.New("insufficient threshold signature shares")
	ErrInvalidSignerIdx   = errors.New("invalid signer index")
	ErrDuplicatedSigner   = errors.New("duplicated signer")
)

// InvalidSignatureIncludedError indicates that some signatures, included via TrustedAdd, are invalid
type InvalidSignatureIncludedError struct {
	err error
}

func NewInvalidSignatureIncludedErrorf(msg string, args ...interface{}) error {
	return InvalidSignatureIncludedError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InvalidSignatureIncludedError) Error() string { return e.err.Error() }
func (e InvalidSignatureIncludedError) Unwrap() error { return e.err }

// IsInvalidSignatureIncludedError returns whether err is an InvalidSignatureIncludedError
func IsInvalidSignatureIncludedError(err error) bool {
	var e InvalidSignatureIncludedError
	return errors.As(err, &e)
}
