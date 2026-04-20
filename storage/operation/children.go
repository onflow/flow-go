package operation

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexNewBlock populates the parent-child index for block, by adding the given blockID to the set of children of its parent.
//
// CAUTION:
//   - This function should only be used for KNOWN BLOCKs (neither existence of the block nor its parent is verified here)
//   - The caller must acquire the [storage.LockInsertBlock] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the blockID is already indexed as a child of the parent
func IndexNewBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, parentID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	return indexBlockByParent(rw, blockID, parentID)
}

// IndexNewClusterBlock populates the parent-child index for cluster blocks, aka collections, by adding the given
// blockID to the set of children of its parent.
//
// CAUTION:
//   - This function should only be used for KNOWN BLOCKs (neither existence of the block nor its parent is verified here)
//   - The caller must acquire the [storage.LockInsertOrFinalizeClusterBlock] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if the blockID is already indexed as a child of the parent
func IndexNewClusterBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, parentID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	return indexBlockByParent(rw, blockID, parentID)
}

// indexBlockByParent is the internal function that implements the indexing logic for both regular blocks and cluster blocks.
// the caller must ensure the required locks are held to prevent concurrent writes to the [codeBlockChildren] key space.
func indexBlockByParent(rw storage.ReaderBatchWriter, blockID flow.Identifier, parentID flow.Identifier) error {
	// By convention, the parentID being [flow.ZeroID] means that the block is a root block that has no parent.
	// This is the case for genesis blocks and cluster root blocks. In this case, we don't need to index anything.
	if parentID == flow.ZeroID {
		return nil
	}

	// If the parent block is not zero, depending on whether the parent block has
	// children or not, we will either update the index or insert the index.
	var childrenIDs flow.IdentifierList
	err := RetrieveBlockChildren(rw.GlobalReader(), parentID, &childrenIDs)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not look up block children: %w", err)
		}
	}

	// check we don't add a duplicate
	if slices.Contains(childrenIDs, blockID) {
		return storage.ErrAlreadyExists
	}

	// adding the new block to be another child of the parent
	childrenIDs = append(childrenIDs, blockID)

	// saving the index
	err = UpsertByKey(rw.Writer(), MakePrefix(codeBlockChildren, parentID), childrenIDs)
	if err != nil {
		return fmt.Errorf("could not update children index: %w", err)
	}

	return nil
}

// RetrieveBlockChildren retrieves the list of child block IDs for the specified parent block.
//
// No error returns expected during normal operations.
// It returns [storage.ErrNotFound] if the block has no children.
// Note, this would mean either the block does not exist or the block exists but has no children.
// The caller has to check if the block exists by other means if needed.
func RetrieveBlockChildren(r storage.Reader, blockID flow.Identifier, childrenIDs *flow.IdentifierList) error {
	err := RetrieveByKey(r, MakePrefix(codeBlockChildren, blockID), childrenIDs)
	if err != nil {
		// when indexing new block, we don't create an index for the block if it has no children
		// so we can't distinguish between a block that doesn't exist and a block that exists but has no children
		// If the block doesn't have a children index yet, it means it has no children
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("the block has no children, but it might also be the case the block does not exist: %w", err)
		}

		return fmt.Errorf("could not retrieve block children: %w", err)
	}

	if len(*childrenIDs) == 0 {
		return fmt.Errorf("the block has no children: %w", storage.ErrNotFound)
	}

	return nil
}
