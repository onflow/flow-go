package procedure

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// IndexNewBlock and IndexNewClusterBlock will add parent-child index for the new block.
//   - Each block has a parent, we use this parent-child relationship to build a reverse index
//     for looking up children blocks for a given block. This is useful for forks recovery
//     where we want to find all the pending children blocks for the lastest finalized block.
//
// When adding parent-child index for a new block, we will update two indexes:
//  1. Per protocol convention, blocks must be ingested in "ancestors-first" order. Hence,
//     if a block is new (needs to be verified, to avoid state corruption in case of repeated
//     calls), its set of persisted children is empty at the time of insertion.
//  2. Since the parent block has this new block as a child, we add to the parent block's children.
//     There are two special cases for (2):
//     - If the parent block is zero (i.e. genesis block), then we don't need to add this index.
//     - If the parent block doesn't exist, then we will index the new block as the only child
//     of the parent anyway. This is useful for bootstrapping nodes with truncated history.
func IndexNewBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, parentID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	return insertNewBlock(lctx, rw, blockID, parentID)
}

func IndexNewClusterBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, parentID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	return insertNewBlock(lctx, rw, blockID, parentID)
}

func insertNewBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, parentID flow.Identifier) error {
	// Step 1: index the child for the new block.
	// the new block has no child, so adding an empty child index for it
	err := operation.UpsertBlockChildren(lctx, rw.Writer(), blockID, nil)
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
	var childrenIDs flow.IdentifierList
	err = operation.RetrieveBlockChildren(rw.GlobalReader(), parentID, &childrenIDs)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not look up block children: %w", err)
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
	err = operation.UpsertBlockChildren(lctx, rw.Writer(), parentID, childrenIDs)
	if err != nil {
		return fmt.Errorf("could not update children index: %w", err)
	}

	return nil
}

// LookupBlockChildren looks up the IDs of all child blocks of the given parent block.
func LookupBlockChildren(r storage.Reader, blockID flow.Identifier, childrenIDs *flow.IdentifierList) error {
	return operation.RetrieveBlockChildren(r, blockID, childrenIDs)
}
