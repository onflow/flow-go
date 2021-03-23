package errors

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ContractNotFoundError
type ContractNotFoundError struct {
	Address  flow.Address
	Contract string
}

func (e *ContractNotFoundError) Code() uint32 {
	return errCodeAccountNotFoundError
}

func (e *ContractNotFoundError) Error() string {
	return fmt.Sprintf(
		"contract %s not found for address %s",
		e.Contract,
		e.Address,
	)
}

func (e *ContractNotFoundError) Is(target error) bool {
	_, ok := target.(*ContractNotFoundError)
	return ok
}
