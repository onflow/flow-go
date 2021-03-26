package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ContractNotFoundError is returned when an account contract is not found
type ContractNotFoundError struct {
	address  flow.Address
	contract string
}

// NewContractNotFoundError constructs a new ContractNotFoundError
func NewContractNotFoundError(address flow.Address, contract string) error {
	return &ContractNotFoundError{
		address:  address,
		contract: contract,
	}
}

func (e *ContractNotFoundError) Error() string {
	return fmt.Sprintf(
		"contract %s not found for address %s",
		e.contract,
		e.address,
	)
}

// Code returns the error code for this error type
func (e *ContractNotFoundError) Code() uint32 {
	return errCodeContractNotFoundError
}

// ContractNamesNotFoundError is returned when fetching a list of contract names under an account
type ContractNamesNotFoundError struct {
	address flow.Address
}

// NewContractNamesNotFoundError constructs a new ContractNamesNotFoundError
func NewContractNamesNotFoundError(address flow.Address) error {
	return &ContractNamesNotFoundError{
		address: address,
	}
}

func (e *ContractNamesNotFoundError) Error() string {
	return fmt.Sprintf(
		"cannot retrieve current contract names for account %s",
		e.address,
	)
}

// Code returns the error code for this error type
func (e *ContractNamesNotFoundError) Code() uint32 {
	return errCodeContractNamesNotFoundError
}
