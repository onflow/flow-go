package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// AccountNotFoundError is returned when account doesn't exist for the given address
type AccountNotFoundError struct {
	Address flow.Address
}

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf(
		"account not found for address %s",
		e.Address,
	)
}

// Code returns the error code for this error type
func (e *AccountNotFoundError) Code() uint32 {
	return errCodeAccountNotFoundError
}

// Is returns true if the given error type is AccountNotFoundError
func (e *AccountNotFoundError) Is(target error) bool {
	_, ok := target.(*AccountNotFoundError)
	return ok
}

// AccountAlreadyExistsError is returned when account creation fails because
// another account already exist at that address
type AccountAlreadyExistsError struct {
	Address flow.Address
}

func (e *AccountAlreadyExistsError) Error() string {
	return fmt.Sprintf(
		"account with address %s already exists",
		e.Address,
	)
}

// Code returns the error code for this error type
func (e *AccountAlreadyExistsError) Code() uint32 {
	return errCodeAccountAlreadyExistsError
}

// Is returns true if the given error type is AccountAlreadyExistsError
func (e *AccountAlreadyExistsError) Is(target error) bool {
	_, ok := target.(*AccountAlreadyExistsError)
	return ok
}

// AccountPublicKeyNotFoundError is returned when a public key not found for the given address and key index
type AccountPublicKeyNotFoundError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *AccountPublicKeyNotFoundError) Error() string {
	return fmt.Sprintf(
		"account public key not found for address %s and key index %d",
		e.Address,
		e.KeyIndex,
	)
}

// Code returns the error code for this error type
func (e *AccountPublicKeyNotFoundError) Code() uint32 {
	return errCodeAccountPublicKeyNotFoundError
}

// Is returns true if the given error type is AccountPublicKeyNotFoundError
func (e *AccountPublicKeyNotFoundError) Is(target error) bool {
	_, ok := target.(*AccountPublicKeyNotFoundError)
	return ok
}

// FrozenAccountError is returned when a frozen account signs a transaction
type FrozenAccountError struct {
	Address flow.Address
}

func (e *FrozenAccountError) Error() string {
	return fmt.Sprintf("account %s is frozen", e.Address)
}

// Code returns the error code for this error type
func (e *FrozenAccountError) Code() uint32 {
	return errCodeFrozenAccountError
}

// Is returns true if the given error type is FrozenAccountError
func (e *FrozenAccountError) Is(target error) bool {
	_, ok := target.(*FrozenAccountError)
	return ok
}
