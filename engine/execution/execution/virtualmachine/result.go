package virtualmachine

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TxHash  crypto.Hash
	Error   error
	GasUsed uint64
}

func (r TransactionResult) Succeeded() bool {
	return r.Error == nil
}
