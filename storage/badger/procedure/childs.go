package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// IndexChildByBlockID add an index to a block's parent
// child block means that a block is its parent block's child block
// This procedure doesn't check the existance of the child block in storage
func IndexChildByBlockID(blockID flow.Identifier, childID flow.Identifier) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		// index this child
		err := operation.IndexBlockByParentID(blockID, childID)(tx)

		// if there is already a child for this block, then skip
		if err == storage.ErrAlreadyExists {
			return nil
		}

		if err != nil {
			return fmt.Errorf("could not insert child %v to index: %v, %w", childID, blockID, err)
		}

		return nil
	}
}

func RetrieveChildByBlockID(blockID flow.Identifier, childHeader *flow.Header) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		var childID flow.Identifier

		// retrieve the child block ID
		err := operation.LookupBlockIDByParentID(blockID, &childID)(tx)

		if err != nil {
			return fmt.Errorf("could not retrieve child for block: %v, %w", blockID, err)
		}

		// retrieve the block header of the child block
		err = operation.RetrieveHeader(childID, childHeader)(tx)
		// must be able to find child block's header
		if err != nil {
			return fmt.Errorf("could not find child block's header (%v): %w", childID, err)
		}

		return nil
	}
}
