package errors

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// AccountNotFoundError is returned when account doesn't exist for the given address
type AccountNotFoundError struct {
	address flow.Address
}

// NewAccountNotFoundError constructs a new AccountNotFoundError
func NewAccountNotFoundError(address flow.Address) error {
	return &AccountNotFoundError{
		address: address,
	}
}

func (e AccountNotFoundError) Error() string {
	return fmt.Sprintf(
		"%s account not found for address %s",
		e.Code().String(),
		e.address.String(),
	)
}

// Code returns the error code for this error type
func (e AccountNotFoundError) Code() ErrorCode {
	return ErrCodeAccountNotFoundError
}

// IsAccountNotFoundError returns true if error has this type
func IsAccountNotFoundError(err error) bool {
	var t *AccountNotFoundError
	return errors.As(err, &t)
}

// AccountAlreadyExistsError is returned when account creation fails because
// another account already exist at that address
// TODO maybe this should be failure since user has no control over this
type AccountAlreadyExistsError struct {
	address flow.Address
}

// NewAccountAlreadyExistsError constructs a new AccountAlreadyExistsError
func NewAccountAlreadyExistsError(address flow.Address) error {
	return &AccountAlreadyExistsError{address: address}
}

func (e AccountAlreadyExistsError) Error() string {
	return fmt.Sprintf(
		"%s account with address %s already exists",
		e.Code().String(),
		e.address,
	)
}

// Code returns the error code for this error type
func (e AccountAlreadyExistsError) Code() ErrorCode {
	return ErrCodeAccountAlreadyExistsError
}

// AccountPublicKeyNotFoundError is returned when a public key not found for the given address and key index
type AccountPublicKeyNotFoundError struct {
	address  flow.Address
	keyIndex uint64
}

// NewAccountPublicKeyNotFoundError constructs a new AccountPublicKeyNotFoundError
func NewAccountPublicKeyNotFoundError(address flow.Address, keyIndex uint64) *AccountPublicKeyNotFoundError {
	return &AccountPublicKeyNotFoundError{address: address, keyIndex: keyIndex}
}

// IsAccountAccountPublicKeyNotFoundError returns true if error has this type
func IsAccountAccountPublicKeyNotFoundError(err error) bool {
	var t *AccountPublicKeyNotFoundError
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
func NewFrozenAccountError(address flow.Address) error {
	return &FrozenAccountError{address: address}
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
