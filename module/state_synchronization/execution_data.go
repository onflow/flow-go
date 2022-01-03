package state_synchronization

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// ExecutionData represents the execution data of a block
type ExecutionData struct {
	BlockID            flow.Identifier
	Collections        []*flow.Collection
	Events             []*flow.Event
	TrieUpdates        []*ledger.TrieUpdate
	TransactionResults []*flow.TransactionResult
}
