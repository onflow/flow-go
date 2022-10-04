package errors

import (
	"github.com/onflow/flow-go/model/flow"
)

// NewContractNotFoundError constructs a new ErrContractNotFoundError error
func NewContractNotFoundError(
	address flow.Address,
	contract string,
) CodedError {
	return NewCodedError(
		ErrCodeContractNotFoundError,
		"contract %s not found for address %s",
		contract,
		address)
}
