package executor

import "github.com/dapperlabs/flow-go/model/flow"

// An ExecutableBlock contains the block information, collection guarantees and transactions required
// to execute a single block.
type ExecutableBlock struct {
	Block            *flow.Block
	Collections      []*ExecutableCollection
	PreviousResultID flow.Identifier
}

type ExecutableCollection struct {
	Guarantee    *flow.CollectionGuarantee
	Transactions []*flow.TransactionBody
}
