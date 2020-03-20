package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertExecutionStateViews(blockID flow.Identifier, views []*delta.View) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionStateView, blockID), views)
}

func RetrieveExecutionStateViews(blockID flow.Identifier, views []*delta.View) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionStateView, blockID), views)
}
