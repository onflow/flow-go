package errors

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

func NewAccountNotFoundError(address flow.Address) *CodedError {
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
func NewAccountAlreadyExistsError(address flow.Address) *CodedError {
	return NewCodedError(
		ErrCodeAccountAlreadyExistsError,
		"account with address %s already exists",
		address)
}

// AccountPublicKeyNotFoundError is returned when a public key not found for the given address and key index
type AccountPublicKeyNotFoundError struct {
	address  flow.Address
	keyIndex uint64
}

// NewAccountPublicKeyNotFoundError constructs a new AccountPublicKeyNotFoundError
func NewAccountPublicKeyNotFoundError(address flow.Address, keyIndex uint64) AccountPublicKeyNotFoundError {
	return AccountPublicKeyNotFoundError{address: address, keyIndex: keyIndex}
}

// IsAccountAccountPublicKeyNotFoundError returns true if error has this type
func IsAccountAccountPublicKeyNotFoundError(err error) bool {
	var t AccountPublicKeyNotFoundError
	return errors.As(err, &t)
}

func (e AccountPublicKeyNotFoundError) Error() string {
	return fmt.Sprintf(
		"%s account public key not found for address %s and key index %d",
		e.Code().String(),
		e.address,
		e.keyIndex,
	)
}

// Code returns the error code for this error type
func (e AccountPublicKeyNotFoundError) Code() ErrorCode {
	return ErrCodeAccountPublicKeyNotFoundError
}

// FrozenAccountError is returned when a frozen account signs a transaction
type FrozenAccountError struct {
	address flow.Address
}

// NewFrozenAccountError constructs a new FrozenAccountError
func NewFrozenAccountError(address flow.Address) FrozenAccountError {
	return FrozenAccountError{address: address}
}

// Address returns the address of frozen account
func (e FrozenAccountError) Address() flow.Address {
	return e.address
}

func (e FrozenAccountError) Error() string {
	return fmt.Sprintf("%s account %s is frozen", e.Code().String(), e.address)
}

// Code returns the error code for this error type
func (e FrozenAccountError) Code() ErrorCode {
	return ErrCodeFrozenAccountError
}

// NewAccountPublicKeyLimitError constructs a new CodedError.  It is returned
// when an account tries to add public keys over the limit.
func NewAccountPublicKeyLimitError(
	address flow.Address,
	counts uint64,
	limit uint64,
) *CodedError {
	return NewCodedError(
		ErrCodeAccountPublicKeyLimitError,
		"account's (%s) public key count (%d) exceeded the limit (%d)",
		address,
		counts,
		limit)
}
