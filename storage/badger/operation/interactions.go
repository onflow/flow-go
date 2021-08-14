package operation

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"
)

func InsertExecutionStateInteractions(blockID flow.Identifier, interactions []*delta.Snapshot) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionStateInteractions, blockID), interactions)
}

func RetrieveExecutionStateInteractions(blockID flow.Identifier, interactions *[]*delta.Snapshot) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionStateInteractions, blockID), interactions)
}
