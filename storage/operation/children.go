package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexNewBlock indexes a new block and updates the parent-child relationship in the block children index.
// This function creates an empty children index for the new block and adds the new block to the parent's children list.
//
// CAUTION:
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

// IndexNewClusterBlock indexes a new cluster block and updates the parent-child relationship in the block children index.
// This function creates an empty children index for the new cluster block and adds the new block to the parent's children list.
//
// CAUTION:
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

func indexBlockByParent(rw storage.ReaderBatchWriter, blockID flow.Identifier, parentID flow.Identifier) error {
	// Step 1: make sure the new block has no children yet
	var nonExist flow.IdentifierList
	err := RetrieveBlockChildren(rw.GlobalReader(), blockID, &nonExist)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not check for existing children of new block: %w", err)
		}
	}

	if len(nonExist) > 0 {
		return fmt.Errorf("a new block supposed to have no children, but found %v: %w", nonExist,
			storage.ErrAlreadyExists)
	}

	// Step 2: adding the second index for the parent block
	// if the parent block is zero, for instance root block has no parent,
	// then no need to add index for it
	// useful to skip for cluster root block which has no parent
	if parentID == flow.ZeroID {
		return nil
	}

	// if the parent block is not zero, depending on whether the parent block has
	// children or not, we will either update the index or insert the index:
	// when parent block doesn't exist, we will insert the block children.
	// when parent block exists already, we will update the block children,
	var childrenIDs flow.IdentifierList
	err = RetrieveBlockChildren(rw.GlobalReader(), parentID, &childrenIDs)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not look up block children: %w", err)
		}
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
	err = UpsertByKey(rw.Writer(), MakePrefix(codeBlockChildren, parentID), childrenIDs)
	if err != nil {
		return fmt.Errorf("could not update children index: %w", err)
	}

	return nil
}

// RetrieveBlockChildren retrieves the list of child block IDs for the specified parent block.
//
// Expected errors during normal operations:
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
