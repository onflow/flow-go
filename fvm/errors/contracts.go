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
func NewContractNotFoundError(address flow.Address, contract string) ContractNotFoundError {
	return ContractNotFoundError{
		address:  address,
		contract: contract,
	}
}

func (e ContractNotFoundError) Error() string {
	return fmt.Sprintf(
		"%s contract %s not found for address %s",
		e.Code().String(),
		e.contract,
		e.address,
	)
}

// Code returns the error code for this error type
func (e ContractNotFoundError) Code() ErrorCode {
	return ErrCodeContractNotFoundError
}
