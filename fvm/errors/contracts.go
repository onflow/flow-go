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
		"%s contract %s not found for address %s",
		e.Code().String(),
		e.contract,
		e.address,
	)
}

// Code returns the error code for this error type
func (e *ContractNotFoundError) Code() ErrorCode {
	return ErrCodeContractNotFoundError
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
		"%s cannot retrieve current contract names for account %s",
		e.Code().String(),
		e.address,
	)
}

// Code returns the error code for this error type
func (e *ContractNamesNotFoundError) Code() ErrorCode {
	return ErrCodeContractNamesNotFoundError
}

// ContractCompressionError is returned when de/compressing contract codes fails
type ContractCompressionError struct {
	address flow.Address
	name    string
}

// NewContractCompressionError constructs a new ContractCompressionError
func NewContractCompressionError(address flow.Address, name string) error {
	return &ContractCompressionError{
		address: address,
		name:    name,
	}
}

func (e *ContractCompressionError) Error() string {
	return fmt.Sprintf(
		"%s cannot compress or decompress contract code with name %s for account %s",
		e.Code().String(),
		e.name,
		e.address,
	)
}

// Code returns the error code for this error type
func (e *ContractCompressionError) Code() ErrorCode {
	return ErrCodeContractCompressionError
}
