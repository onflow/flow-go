package errors

import (
	"github.com/onflow/flow-go/model/flow"
)

func NewAccountNotFoundError(address flow.Address) CodedError {
	return NewCodedError(
		ErrCodeAccountNotFoundError,
		"account not found for address %s",
		address.String())
}

// IsAccountNotFoundError returns true if error has this type
func IsAccountNotFoundError(err error) bool {
	return HasErrorCode(err, ErrCodeAccountNotFoundError)
}

// NewAccountAlreadyExistsError constructs a new CodedError. It is returned
// when account creation fails because another account already exist at that
// address.
//
// TODO maybe this should be failure since user has no control over this
func NewAccountAlreadyExistsError(address flow.Address) CodedError {
	return NewCodedError(
		ErrCodeAccountAlreadyExistsError,
		"account with address %s already exists",
		address)
}

// NewAccountPublicKeyNotFoundError constructs a new CodedError. It is returned
// when a public key not found for the given address and key index.
func NewAccountPublicKeyNotFoundError(
	address flow.Address,
	keyIndex uint32,
) CodedError {
	return NewCodedError(
		ErrCodeAccountPublicKeyNotFoundError,
		"account public key not found for address %s and key index %d",
		address,
		keyIndex)
}

// IsAccountPublicKeyNotFoundError returns true if error has this type
func IsAccountPublicKeyNotFoundError(err error) bool {
	return HasErrorCode(err, ErrCodeAccountPublicKeyNotFoundError)
}

// NewAccountPublicKeyLimitError constructs a new CodedError.  It is returned
// when an account tries to add public keys over the limit.
func NewAccountPublicKeyLimitError(
	address flow.Address,
	counts uint32,
	limit uint32,
) CodedError {
	return NewCodedError(
		ErrCodeAccountPublicKeyLimitError,
		"account's (%s) public key count (%d) exceeded the limit (%d)",
		address,
		counts,
		limit)
}
