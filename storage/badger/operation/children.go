package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// IndexBlockByParentID insert an index to lookup the direct child of a block by its ID
func IndexBlockByParentID(parent flow.Identifier, child flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexChild, parent), child)
}

// LookupBlockIDByParentID the child block ID by parent block ID
func LookupBlockIDByParentID(parent flow.Identifier, child *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexChild, parent), child)
}
