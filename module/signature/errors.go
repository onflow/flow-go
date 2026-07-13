package signature

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidSignatureFormat = errors.New("signature's binary format is invalid")

	ErrInsufficientShares = errors.New("insufficient threshold signature shares")

	// ErrIncompatibleBitVectorLength indicates that the bit vector's length is different than
	// the expected length, based on the supplied node list.
	ErrIncompatibleBitVectorLength = errors.New("bit vector has incompatible length")

	// ErrIllegallyPaddedBitVector indicates that the index vector was padded with unexpected bit values.
	ErrIllegallyPaddedBitVector = errors.New("index vector padded with unexpected bit values")

	// ErrInvalidChecksum indicates that the index vector's checksum is invalid
	ErrInvalidChecksum = errors.New("index vector's checksum is invalid")

	// ErrIdentityPublicKey indicates that the signer's public keys add up to the BLS identity public key.
	// Any signature would fail the cryptographic verification if verified against the
	// the identity public key. This case can only happen if public keys were forged to sum up to
	// an identity public key. If private keys are sampled uniformly at random, there is vanishing
	// probability of generating the aggregated identity public key. However, (colluding) byzantine
	// signers could force the generation of private keys that result in the identity aggregated key.
	ErrIdentityPublicKey = errors.New("aggregated public key is identity and aggregated signature is invalid")
)

/* ********************* InvalidSignatureIncludedError ********************* */

// InvalidSignatureIncludedError indicates that some signatures, included via TrustedAdd, are invalid
type InvalidSignatureIncludedError struct {
	err error
}

func NewInvalidSignatureIncludedErrorf(msg string, args ...any) error {
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

func NewInvalidSignerIdxErrorf(msg string, args ...any) error {
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

func NewDuplicatedSignerIdxErrorf(msg string, args ...any) error {
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

func NewInsufficientSignaturesErrorf(msg string, args ...any) error {
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

/* ********************** InvalidSignerIndicesError ********************** */

// InvalidSignerIndicesError indicates that a bit vector does not encode a valid set of signers
type InvalidSignerIndicesError struct {
	err error
}

func NewInvalidSignerIndicesErrorf(msg string, args ...any) error {
	return InvalidSignerIndicesError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InvalidSignerIndicesError) Error() string { return e.err.Error() }
func (e InvalidSignerIndicesError) Unwrap() error { return e.err }

// IsInvalidSignerIndicesError returns whether err is an InvalidSignerIndicesError
func IsInvalidSignerIndicesError(err error) bool {
	var e InvalidSignerIndicesError
	return errors.As(err, &e)
}

/* ********************** InvalidSignerIndicesError ********************** */

// InvalidSigTypesError indicates that the given data not encode valid signature types
type InvalidSigTypesError struct {
	err error
}

func NewInvalidSigTypesErrorf(msg string, args ...any) error {
	return InvalidSigTypesError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InvalidSigTypesError) Error() string { return e.err.Error() }
func (e InvalidSigTypesError) Unwrap() error { return e.err }

// IsInvalidSigTypesError returns whether err is an InvalidSigTypesError
func IsInvalidSigTypesError(err error) bool {
	var e InvalidSigTypesError
	return errors.As(err, &e)
}
