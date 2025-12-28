package operation

import (
	"errors"
	"fmt"

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
	// if !lctx.HoldsLock(storage.LockInsertBlock) {
	// 	return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	// }
	//
	// return indexBlockByParent(rw, blockID, parentID)

	return ErrChainArchived
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
	// if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
	// 	return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	// }
	//
	// return indexBlockByParent(rw, blockID, parentID)

	return ErrChainArchived
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
// No error returns expected during normal operations.
// It returns [storage.ErrNotFound] if the block has no children.
// Note, this would mean either the block does not exist or the block exists but has no children.
// The caller has to check if the block exists by other means if needed.
//
// [BeyondArchiveThresholdError] wrapping [storage.ErrNotFound] is returned in the following situations:
//  0. If `blockID` does not exist in the database at all, we return [storage.ErrNotFound] (consistent with other behaviour).
//  1. If `blockID` is beyond the archive threshold, we get [storage.ErrNotFound] wrapped in [BeyondArchiveThresholdError]
//     (extension of the old behaviour, downwards compatible).
//  2. If any of the indexed children of `blockID` is beyond the archive threshold, it is excluded from `childrenIDs`.
//     (extension of the old behaviour, downwards compatible).
//  3. If all children of `blockID` are beyond the archive threshold, [RetrieveBlockChildren] emulates the situation where
//     a known block has no known children. In that case, the index is simply not populated in the database, resulting in
//     a [storage.ErrNotFound]. Depending on no children are indexed in the database or all children are beyond the archive
//     threshold, we return either a brare [storage.ErrNotFound] or one wrapped in [BeyondArchiveThresholdError] respectively.
//     (extension of the old behaviour, downwards compatible).
func RetrieveBlockChildren(r storage.Reader, blockID flow.Identifier, childrenIDs *flow.IdentifierList) error {
	// ARCHIVE THRESHOLD: This code is intended to withold blocks beyond the view of a "latest finalized block". We simply
	// pretend those blocks do not exist, which emulates a situation where the node has not yet received those blocks.
	var h flow.Header
	err := RetrieveHeader(r, blockID, &h)
	if err != nil {
		// RetrieveHeader returns [BeyondArchiveThresholdError] if block is in the database but beyond the archive threshold.
		// A [storage.ErrNotFound] is returned if the block is not in the database at all. We can just propagate these errors:
		return err
	} // block is known, i.e. confirmed to be within archive boundaries

	// CAUTION: the following index was written at a time when the block's children were still actively consumed. We
	// don't want to alter the data, just pretend the children beyond the archive's boundary haven't been received yet.
	var unsaveDescendantIDs flow.IdentifierList
	err = RetrieveByKey(r, MakePrefix(codeBlockChildren, blockID), &unsaveDescendantIDs)
	if err != nil {
		// when indexing new block, we don't create an index for the block if it has no children
		// so we can't distinguish between a block that doesn't exist and a block that exists but has no children
		// If the block doesn't have a children index yet, it means it has no children
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("the block has no children, but it might also be the case the block does not exist: %w", err)
		}

		return fmt.Errorf("could not retrieve block children: %w", err)
	}

	// we have retrieved some slice of children IDs. It shouldn't be empty, but just in case it is, we return [storage.ErrNotFound] for consistency.
	if len(unsaveDescendantIDs) == 0 {
		return fmt.Errorf("the block has no children: %w", storage.ErrNotFound)
	}

	// filtering out children that are beyond archive threshold
	var filteredDescendantIDs flow.IdentifierList
	for _, childID := range unsaveDescendantIDs {
		var childHeader flow.Header
		err = RetrieveHeader(r, childID, &childHeader)
		if err != nil {
			// This is an over-permissive check: we utilize the fact that any [BeyondArchiveThresholdError] returned by [RetrieveHeader]
			// _wraps_ a [storage.ErrNotFound] error. Thus, we can simply check for broader class of [storage.ErrNotFound] sentinels here.
			if errors.Is(err, storage.ErrNotFound) {
				continue // child block is beyond archive threshold: pretend it does not exist
			}
			return fmt.Errorf("could not retrieve child block header: %w", err)
		}

		filteredDescendantIDs = append(filteredDescendantIDs, childID) // child block is within archive threshold, keep it
	}

	// After filtering, the slice might be empty. Then, we return [storage.ErrNotFound] for consistency. This which emulates the situation
	// where a known block has no known children. However, as there were children before, we wrap it in an [BeyondArchiveThresholdError].
	if len(filteredDescendantIDs) == 0 {
		return NewBeyondArchiveThresholdf("block has no children within the archive's boundary: %w", storage.ErrNotFound)
	}

	*childrenIDs = filteredDescendantIDs
	return nil
}
