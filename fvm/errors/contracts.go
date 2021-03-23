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
	return errCodeAccountNotFoundError
}

// Is returns true if the given error type is ContractNotFoundError
func (e *ContractNotFoundError) Is(target error) bool {
	_, ok := target.(*ContractNotFoundError)
	return ok
}
