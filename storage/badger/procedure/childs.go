package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

func IndexChildByBlockID(blockID flow.Identifier, childID flow.Identifier) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		childID := flow.Identifier{}

		err := operation.RetrieveChild(blockID, &childID)(tx)
		if err == nil {
			// there is a child exist already abort
			return nil
		}

		if err != badger.ErrKeyNotFound {
			return fmt.Errorf("could not retrieve child for block: %v, %w", blockID, err)
		}

		// there is no child for this block
		// index this child
		err = operation.InsertChild(blockID, childID)(tx)
		if err != nil {
			return fmt.Errorf("could not insert child to index: %v, %w", blockID, err)
		}

		return nil
	}
}

func RetrieveChildByBlockID(blockID flow.Identifier, childHeader *flow.Header) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {

		childID := flow.Identifier{}

		// retrieve the child block ID
		err := operation.RetrieveChild(blockID, &childID)(tx)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return storage.ErrNotFound
			}
			return fmt.Errorf("could not retrieve child for block: %v, %w", blockID, err)
		}

		// retrieve the block header of the child block
		err = operation.RetrieveHeader(childID, childHeader)(tx)
		if err != nil {
			return fmt.Errorf("could not child block's header: %w", err)
		}

		return nil
	}
}
