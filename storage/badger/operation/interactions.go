package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertExecutionStateInteractions(blockID flow.Identifier, interactions []*delta.Snapshot) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionStateInteractions, blockID), interactions)
}

func RetrieveExecutionStateInteractions(blockID flow.Identifier, interactions *[]*delta.Snapshot) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionStateInteractions, blockID), interactions)
}
