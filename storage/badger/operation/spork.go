package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertSporkRootBlock inserts the spork root block for the present spork. A single database
// and protocol state instance spans at most one spork, so this is inserted
// exactly once, when bootstrapping the state.
func InsertSporkRootBlock(block *flow.Block) func(*badger.Txn) error {
	return insert(makePrefix(codeSporkRootBlock), block)
}

// RetrieveSporkRootBlock retrieves the spork root block for the present spork.
func RetrieveSporkRootBlock(block *flow.Block) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSporkRootBlock), block)
}
