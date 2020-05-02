package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertBlockChildren insert an index to lookup the direct child of a block by its ID
func InsertBlockChildren(blockID flow.Identifier, childrenIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexChildren, blockID), childrenIDs)
}

// UpdateBlockChildren updates the children for a block.
func UpdateBlockChildren(blockID flow.Identifier, childrenIDs []flow.Identifier) func(*badger.Txn) error {
	return update(makePrefix(codeIndexChildren, blockID), childrenIDs)
}

// RetrieveBlockChildren the child block ID by parent block ID
func RetrieveBlockChildren(blockID flow.Identifier, childrenIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexChildren, blockID), childrenIDs)
}
