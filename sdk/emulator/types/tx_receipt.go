package types

import (
	"github.com/dapperlabs/flow-go/crypto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// TransactionReceipt describes the result of a transaction execution.
type TransactionReceipt struct {
	TransactionHash crypto.Hash
	Status          flow.TransactionStatus
	Error           error
	Events          []flow.Event
}

func (result TransactionReceipt) Succeeded() bool {
	return result.Status == flow.TransactionFinalized
}

func (result TransactionReceipt) Reverted() bool {
	return result.Status == flow.TransactionReverted
}
