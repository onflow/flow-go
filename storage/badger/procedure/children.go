package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// IndexBlockChild adds a child block to the index of its parent block.
func IndexBlockChild(blockID flow.Identifier, childID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the current children
		var childrenIDs []flow.Identifier
		err := operation.RetrieveBlockChildren(blockID, &childrenIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not look up block children: %w", err)
		}

		// check we don't add a duplicate
		for _, dupID := range childrenIDs {
			if childID == dupID {
				return storage.ErrAlreadyExists
			}
		}

		// add the child ID and store update
		childrenIDs = append(childrenIDs, childID)
		err = operation.UpdateBlockChildren(blockID, childrenIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not update children index: %w", err)
		}

		return nil
	}
}

// LookupBlockChildren looks up the IDs of all child blocks of the given parent block.
func LookupBlockChildren(blockID flow.Identifier, childrenIDs *[]flow.Identifier) func(tx *badger.Txn) error {
	return operation.RetrieveBlockChildren(blockID, childrenIDs)
}
