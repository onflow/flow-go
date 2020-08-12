package procedure

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// IndexNewBlock will add parent-child index for the new block.
// - Each block has a parent, we use this parent-child relationship to build a reverse index
// - for looking up children blocks for a given block. This is useful for forks recovery
//   where we want to find all the pending children blocks for the lastest finalized block.
// - when adding parent-child index for a new block, we will add two indexes:
//   1) since it's a new block, the new block should have no child, so adding an empty
//      index for the new block. Note: It's impossible there is a block whose parent is the
//      new block.
//   2) since the parent block has this new block as a child, adding an index for that.
//      there are two special cases for 2):
//      - if the parent block is zero, then we don't need to add this index.
//      - if the parent block doesn't exist, then we will insert the child index instead of
// 				updating
func IndexNewBlock(blockID flow.Identifier, parentID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		// Step 1: index the child for the new block.
		// the new block has no child, so adding an empty child index for it
		err := operation.InsertBlockChildren(blockID, nil)(tx)
		if err != nil {
			return fmt.Errorf("could not insert empty block children: %w", err)
		}

		// Step 2: adding the second index for the parent block
		// if the parent block is zero, for instance root block has no parent,
		// then no need to add index for it
		if parentID == flow.ZeroID {
			return nil
		}

		// if the parent block is not zero, depending on whether the parent block has
		// children or not, we will either update the index or insert the index:
		// when parent block doesn't exist, we will insert the block children.
		// when parent block exists already, we will update the block children,
		var childrenIDs []flow.Identifier
		err = operation.RetrieveBlockChildren(parentID, &childrenIDs)(tx)

		var saveIndex func(blockID flow.Identifier, childrenIDs []flow.Identifier) func(*badger.Txn) error
		if errors.Is(err, storage.ErrNotFound) {
			saveIndex = operation.InsertBlockChildren
		} else if err != nil {
			return fmt.Errorf("could not look up block children: %w", err)
		} else { // err == nil
			saveIndex = operation.UpdateBlockChildren
		}

		// check we don't add a duplicate
		for _, dupID := range childrenIDs {
			if blockID == dupID {
				return storage.ErrAlreadyExists
			}
		}

		// adding the new block to be another child of the parent
		childrenIDs = append(childrenIDs, blockID)

		// saving the index
		err = saveIndex(parentID, childrenIDs)(tx)
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
