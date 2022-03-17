package signature

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidSignatureFormat = errors.New("signature's binary format is invalid")

	ErrInsufficientShares = errors.New("insufficient threshold signature shares")

	// IncompatibleBitVectorLengthError indicates that the bit vector's length is different than
	// the expected length, based on the supplied node list.
	IncompatibleBitVectorLengthError = errors.New("bit vector has incompatible length")

	// IllegallyPaddedBitVectorError indicates that the index vector was padded with unexpected bit values.
	IllegallyPaddedBitVectorError = errors.New("index vector padded with unexpected bit values")
)

/* ********************* InvalidSignatureIncludedError ********************* */

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

/* ************************* InvalidSignerIdxError ************************* */

// InvalidSignerIdxError indicates that the signer index is invalid
type InvalidSignerIdxError struct {
	err error
}

func NewInvalidSignerIdxErrorf(msg string, args ...interface{}) error {
	return InvalidSignerIdxError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InvalidSignerIdxError) Error() string { return e.err.Error() }
func (e InvalidSignerIdxError) Unwrap() error { return e.err }

// IsInvalidSignerIdxError returns whether err is an InvalidSignerIdxError
func IsInvalidSignerIdxError(err error) bool {
	var e InvalidSignerIdxError
	return errors.As(err, &e)
}

/* ************************ DuplicatedSignerIdxError *********************** */

// DuplicatedSignerIdxError indicates that a signature from the respective signer index was already added
type DuplicatedSignerIdxError struct {
	err error
}

func NewDuplicatedSignerIdxErrorf(msg string, args ...interface{}) error {
	return DuplicatedSignerIdxError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e DuplicatedSignerIdxError) Error() string { return e.err.Error() }
func (e DuplicatedSignerIdxError) Unwrap() error { return e.err }

// IsDuplicatedSignerIdxError returns whether err is an DuplicatedSignerIdxError
func IsDuplicatedSignerIdxError(err error) bool {
	var e DuplicatedSignerIdxError
	return errors.As(err, &e)
}

/* ********************** InsufficientSignaturesError ********************** */

// InsufficientSignaturesError indicates that not enough signatures have been stored to complete the operation.
type InsufficientSignaturesError struct {
	err error
}

func NewInsufficientSignaturesErrorf(msg string, args ...interface{}) error {
	return InsufficientSignaturesError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InsufficientSignaturesError) Error() string { return e.err.Error() }
func (e InsufficientSignaturesError) Unwrap() error { return e.err }

// IsInsufficientSignaturesError returns whether err is an InsufficientSignaturesError
func IsInsufficientSignaturesError(err error) bool {
	var e InsufficientSignaturesError
	return errors.As(err, &e)
}
