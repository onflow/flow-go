package virtualmachine

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// A TransactionResult is the result of executing a transaction.
type TransactionResult struct {
	TxID    flow.Identifier
	Error   error
	GasUsed uint64
}

func (r TransactionResult) Succeeded() bool {
	return r.Error == nil
}
