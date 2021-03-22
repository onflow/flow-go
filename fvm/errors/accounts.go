package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// AccountPublicKeyNotFoundError not found for the given address
type AccountNotFoundError struct {
	Address flow.Address
}

func (e *AccountNotFoundError) Code() uint32 {
	return errCodeAccountNotFoundError
}

func (e *AccountNotFoundError) Error() string {
	return fmt.Sprintf(
		"account not found for address %s",
		e.Address,
	)
}

// AccountPublicKeyNotFoundError not found for the given address
type AccountAlreadyExistsError struct {
	Address flow.Address
}

func (e *AccountAlreadyExistsError) Code() uint32 {
	return errCodeAccountAlreadyExistsError
}

func (e *AccountAlreadyExistsError) Error() string {
	return fmt.Sprintf(
		"account with address %s already exists",
		e.Address,
	)
}

// AccountPublicKeyNotFoundError not found for the given address
type AccountPublicKeyNotFoundError struct {
	Address  flow.Address
	KeyIndex uint64
}

func (e *AccountPublicKeyNotFoundError) Code() uint32 {
	return errCodeAccountPublicKeyNotFoundError
}

func (e *AccountPublicKeyNotFoundError) Error() string {
	return fmt.Sprintf(
		"account public key not found for address %s and key index %d",
		e.Address,
		e.KeyIndex,
	)
}

type FrozenAccountError struct {
	Address flow.Address
}

func (e *FrozenAccountError) Error() string {
	return fmt.Sprintf("account %s is frozen", e.Address)
}
