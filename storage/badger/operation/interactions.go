package operation

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"
)

func InsertExecutionStateInteractions(
	blockID flow.Identifier,
	executionSnapshots []*snapshot.ExecutionSnapshot,
) func(*badger.Txn) error {
	return insert(
		makePrefix(codeExecutionStateInteractions, blockID),
		executionSnapshots)
}

func RetrieveExecutionStateInteractions(
	blockID flow.Identifier,
	executionSnapshots *[]*snapshot.ExecutionSnapshot,
) func(*badger.Txn) error {
	return retrieve(
		makePrefix(codeExecutionStateInteractions, blockID), executionSnapshots)
}

func RemoveExecutionStateInteractions(blockID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeExecutionStateInteractions, blockID))
}
