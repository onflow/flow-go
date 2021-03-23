package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ContractNotFoundError is returned when an account contract is not found
type ContractNotFoundError struct {
	Address  flow.Address
	Contract string
}

func (e *ContractNotFoundError) Error() string {
	return fmt.Sprintf(
		"contract %s not found for address %s",
		e.Contract,
		e.Address,
	)
}

// Code returns the error code for this error type
func (e *ContractNotFoundError) Code() uint32 {
	return errCodeContractNotFoundError
}

// Is returns true if the given error type is ContractNotFoundError
func (e *ContractNotFoundError) Is(target error) bool {
	_, ok := target.(*ContractNotFoundError)
	return ok
}

// ContractNamesNotFoundError is returned when fetching a list of contract names under an account
type ContractNamesNotFoundError struct {
	Address flow.Address
}

func (e *ContractNamesNotFoundError) Error() string {
	return fmt.Sprintf(
		"cannot retrieve current contract names for account %s",
		e.Address,
	)
}

// Code returns the error code for this error type
func (e *ContractNamesNotFoundError) Code() uint32 {
	return errCodeContractNamesNotFoundError
}

// Is returns true if the given error type is ContractNamesNotFoundError
func (e *ContractNamesNotFoundError) Is(target error) bool {
	_, ok := target.(*ContractNamesNotFoundError)
	return ok
}
