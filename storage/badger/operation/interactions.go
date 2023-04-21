package operation

import (
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"
)

func InsertExecutionStateInteractions(
	blockID flow.Identifier,
	executionSnapshots []*state.ExecutionSnapshot,
) func(*badger.Txn) error {
	return insert(
		makePrefix(codeExecutionStateInteractions, blockID),
		executionSnapshots)
}

func RetrieveExecutionStateInteractions(
	blockID flow.Identifier,
	executionSnapshots *[]*state.ExecutionSnapshot,
) func(*badger.Txn) error {
	return retrieve(
		makePrefix(codeExecutionStateInteractions, blockID), executionSnapshots)
}
