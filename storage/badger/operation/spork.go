package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertSporkRootBlockID inserts the spork root block ID for the present spork. A single database
// and protocol state instance spans at most one spork, so this is inserted
// exactly once, when bootstrapping the state.
func InsertSporkRootBlockID(blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeSporkRootBlockID), blockID)
}

// RetrieveSporkRootBlockID retrieves the spork root block ID for the present spork.
func RetrieveSporkRootBlockID(blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSporkRootBlockID), blockID)
}
