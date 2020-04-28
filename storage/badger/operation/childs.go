package operation

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dgraph-io/badger/v2"
)

// InsertChild insert an index to lookup the direct child of a block by its ID
func InsertChild(parent flow.Identifier, child flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexChild, parent), child)
}

// RetrieveChild the child block ID by parent block ID
func RetrieveChild(parent flow.Identifier, child *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexChild, parent), child)
}
