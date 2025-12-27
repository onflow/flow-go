package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Suppress unused import warning - fmt is used in commented-out code
var _ = fmt.Errorf

// Archive threshold constants
// MANUALLY SPECIFIED: no consistency checks!!
// It is recommended to set the latest sealed and latest finalized block as they would be under normal protocol operations.
// For correctness, the latest sealed block must be either equal to the latest finalized block or its ancestor. However, it is
// recommended, to avoid pruning seals. Hence, the seal for the latest sealed block should be included either in the latest
// finalized block or one of its ancestors.
// Blocks with views/heights above these thresholds will return storage.ErrNotFound.
var (
	// ArchiveLatestSealedHeight is the height of the latest sealed block in the archived chain.
	ArchiveLatestSealedHeight uint64 = 1000 // TODO: replace with actual value
	// ArchiveLatestSealedView is the view of the latest sealed block in the archived chain.
	ArchiveLatestSealedView uint64 = 1000 // TODO: replace with actual value
	// ArchiveLatestSealedBlockID is the ID of the latest sealed block in the archived chain.
	ArchiveLatestSealedBlockID = flow.Identifier{} // TODO: replace with actual value

	// ArchiveLatestFinalizedHeight is the height of the latest finalized block in the archived chain.
	ArchiveLatestFinalizedHeight uint64 = 1100 // TODO: replace with actual value
	// ArchiveLatestFinalizedView is the view of the latest finalized block in the archived chain.
	ArchiveLatestFinalizedView uint64 = 1100 // TODO: replace with actual value
	// ArchiveLatestFinalizedBlockID is the ID of the latest finalized block in the archived chain.
	ArchiveLatestFinalizedBlockID = flow.Identifier{} // TODO: replace with actual value
)

// ErrChainArchived is returned when attempting to write to an archived chain.
var ErrChainArchived = errors.New("chain has been archived, no extensions allowed")

// InsertHeader inserts a block header into the database.
//
// CAUTION:
//   - The caller must ensure that headerID is a collision-resistant hash of the provided header!
//     Otherwise, data corruption may occur.
//   - The caller must acquire one (but not both) of the following locks and hold it until the database
//     write has been committed: either [storage.LockInsertBlock] or [storage.LockInsertOrFinalizeClusterBlock].
//
// It returns [storage.ErrAlreadyExists] if the header already exists, i.e. we only insert a new header once.
// This error allows the caller to detect duplicate inserts. If the header is stored along with other parts
// of the block in the same batch, similar duplication checks can be skipped for storing other parts of the block.
// No other errors are expected during normal operation.
func InsertHeader(lctx lockctx.Proof, rw storage.ReaderBatchWriter, headerID flow.Identifier, header *flow.Header) error {
	return ErrChainArchived

	// held := lctx.HoldsLock(storage.LockInsertBlock) || lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock)
	// if !held {
	// 	return fmt.Errorf("missing required lock: %s or %s", storage.LockInsertBlock, storage.LockInsertOrFinalizeClusterBlock)
	// }
	//
	// key := MakePrefix(codeHeader, headerID)
	// exist, err := KeyExists(rw.GlobalReader(), key)
	// if err != nil {
	// 	return err
	// }
	// if exist {
	// 	return fmt.Errorf("header already exists: %w", storage.ErrAlreadyExists)
	// }
	//
	// return UpsertByKey(rw.Writer(), key, header)
}

// RetrieveHeader retrieves the header of the block with the specified ID.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no block with the specified `blockID` is known,
//     or if the block's view exceeds the archive threshold.
//   - generic error in case of unexpected failure from the database layer
func RetrieveHeader(r storage.Reader, blockID flow.Identifier, header *flow.Header) error {
	err := RetrieveByKey(r, MakePrefix(codeHeader, blockID), header)
	if err != nil {
		return err
	}
	// Check if block's view is beyond the archived threshold
	if header.View > ArchiveLatestFinalizedView {
		return storage.ErrNotFound
	}
	return nil

	// return RetrieveByKey(r, MakePrefix(codeHeader, blockID), header)
}

// IndexFinalizedBlockByHeight indexes a block by its height. It must ONLY be called on FINALIZED BLOCKS.
//
// CAUTION: The caller must acquire the [storage.LockFinalizeBlock] and hold it until the database
// write has been committed.
//
// This function guarantees that the index is only inserted once for each height. We return
// [storage.ErrAlreadyExists] if an entry for the given height already exists in the database.
// No other errors are expected during normal operation.
func IndexFinalizedBlockByHeight(lctx lockctx.Proof, rw storage.ReaderBatchWriter, height uint64, blockID flow.Identifier) error {
	return ErrChainArchived

	// if !lctx.HoldsLock(storage.LockFinalizeBlock) {
	// 	return fmt.Errorf("missing required lock: %s", storage.LockFinalizeBlock)
	// }
	//
	// var existingID flow.Identifier
	// key := MakePrefix(codeHeightToBlock, height)
	// err := RetrieveByKey(rw.GlobalReader(), key, &existingID)
	// if err == nil {
	// 	return fmt.Errorf("block ID already exists for height %d with existing ID %v, cannot reindex with blockID %v: %w",
	// 		height, existingID, blockID, storage.ErrAlreadyExists)
	// }
	// if !errors.Is(err, storage.ErrNotFound) {
	// 	return fmt.Errorf("failed to check existing block ID for height %d: %w", height, err)
	// }
	//
	// return UpsertByKey(rw.Writer(), key, blockID)
}

// IndexCertifiedBlockByView indexes a CERTIFIED block by its view.
// HotStuff guarantees that there is at most one certified block per view. Note that this does not hold
// for uncertified proposals, as a byzantine leader might produce multiple proposals for the same view.
//
// CAUTION: The caller must acquire the [storage.LockInsertBlock] and hold it until the database write
// has been committed.
//
// Hence, only certified blocks (i.e. blocks that have received a QC) can be indexed!
// Returns [storage.ErrAlreadyExists] if an ID has already been finalized for this view.
// No other errors are expected during normal operation.
func IndexCertifiedBlockByView(lctx lockctx.Proof, rw storage.ReaderBatchWriter, view uint64, blockID flow.Identifier) error {
	return ErrChainArchived

	// if !lctx.HoldsLock(storage.LockInsertBlock) {
	// 	return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	// }
	//
	// var existingID flow.Identifier
	// key := MakePrefix(codeCertifiedBlockByView, view)
	// err := RetrieveByKey(rw.GlobalReader(), key, &existingID)
	// if err == nil {
	// 	return fmt.Errorf("block ID already exists for view %d with existingID %v, cannot reindex with blockID %v: %w",
	// 		view, existingID, blockID, storage.ErrAlreadyExists)
	// }
	// if !errors.Is(err, storage.ErrNotFound) {
	// 	return fmt.Errorf("failed to check existing block ID for view %d: %w", view, err)
	// }
	//
	// return UpsertByKey(rw.Writer(), key, blockID)
}

// LookupBlockHeight retrieves finalized blocks by height.
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no finalized block for the specified height is known,
//     or if the height exceeds the archive threshold.
func LookupBlockHeight(r storage.Reader, height uint64, blockID *flow.Identifier) error {
	// Check if height is beyond the archived threshold
	if height > ArchiveLatestFinalizedHeight {
		return storage.ErrNotFound
	}
	return RetrieveByKey(r, MakePrefix(codeHeightToBlock, height), blockID)

	// return RetrieveByKey(r, MakePrefix(codeHeightToBlock, height), blockID)
}

// LookupCertifiedBlockByView retrieves the certified block by view. (Certified blocks are blocks that have received QC.)
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no certified block for the specified view is known,
//     or if the view exceeds the archive threshold.
func LookupCertifiedBlockByView(r storage.Reader, view uint64, blockID *flow.Identifier) error {
	// Check if view is beyond the archived threshold
	if view > ArchiveLatestFinalizedView {
		return storage.ErrNotFound
	}
	return RetrieveByKey(r, MakePrefix(codeCertifiedBlockByView, view), blockID)

	// return RetrieveByKey(r, MakePrefix(codeCertifiedBlockByView, view), blockID)
}

// BlockExists checks whether the block exists in the database.
// Returns false if the block's view exceeds the archive threshold.
// No errors are expected during normal operation.
func BlockExists(r storage.Reader, blockID flow.Identifier) (bool, error) {
	var header flow.Header
	err := RetrieveByKey(r, MakePrefix(codeHeader, blockID), &header)
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Check if block's view is beyond the archived threshold
	if header.View > ArchiveLatestFinalizedView {
		return false, nil
	}
	return true, nil

	// return KeyExists(r, MakePrefix(codeHeader, blockID))
}

// IndexBlockContainingCollectionGuarantee produces a mapping from the ID of a [flow.CollectionGuarantee] to the block ID containing this guarantee.
//
// CAUTION:
//   - The caller must acquire the lock ??? and hold it until the database write has been committed.
//     TODO: USE LOCK, we want to protect this mapping from accidental overwrites (because the key is not derived from the value via a collision-resistant hash)
//   - A collection can be included in multiple *unfinalized* blocks. However, the implementation
//     assumes a one-to-one map from collection ID to a *single* block ID. This holds for FINALIZED BLOCKS ONLY
//     *and* only in the ABSENCE of BYZANTINE collector CLUSTERS (which the mature protocol must tolerate).
//     Hence, this function should be treated as a temporary solution, which requires generalization
//     (one-to-many mapping) for soft finality and the mature protocol.
//
// Expected errors during normal operations:
// TODO: return [storage.ErrAlreadyExists] or [storage.ErrDataMismatch]
func IndexBlockContainingCollectionGuarantee(w storage.Writer, collID flow.Identifier, blockID flow.Identifier) error {
	return ErrChainArchived

	// return UpsertByKey(w, MakePrefix(codeCollectionBlock, collID), blockID)
}

// LookupBlockContainingCollectionGuarantee retrieves the block containing the [flow.CollectionGuarantee] with the given ID.
//
// CAUTION: A collection can be included in multiple *unfinalized* blocks. However, the implementation
// assumes a one-to-one map from collection ID to a *single* block ID. This holds for FINALIZED BLOCKS ONLY
// *and* only in the ABSENCE of BYZANTINE collector CLUSTERS (which the mature protocol must tolerate).
// Hence, this function should be treated as a temporary solution, which requires generalization
// (one-to-many mapping) for soft finality and the mature protocol.
//
// Expected errors during normal operations:
//   - [storage.ErrNotFound] if no block is known that contains the specified collection ID,
//     or if the block's view exceeds the archive threshold.
func LookupBlockContainingCollectionGuarantee(r storage.Reader, collID flow.Identifier, blockID *flow.Identifier) error {
	err := RetrieveByKey(r, MakePrefix(codeCollectionBlock, collID), blockID)
	if err != nil {
		return err
	}
	// Retrieve the header to check if block's view is beyond the archived threshold
	var header flow.Header
	err = RetrieveByKey(r, MakePrefix(codeHeader, *blockID), &header)
	if err != nil {
		// If we can't retrieve the header, the index is inconsistent - return the block ID anyway
		// This maintains backwards compatibility with existing behavior
		return nil
	}
	if header.View > ArchiveLatestFinalizedView {
		*blockID = flow.Identifier{} // Clear the output
		return storage.ErrNotFound
	}
	return nil

	// return RetrieveByKey(r, MakePrefix(codeCollectionBlock, collID), blockID)
}

// FindHeaders iterates through all headers, calling `filter` on each, and adding
// them to the `found` slice if `filter` returned true.
// Headers with view exceeding the archive threshold are excluded from iteration.
func FindHeaders(r storage.Reader, filter func(header *flow.Header) bool, found *[]flow.Header) error {
	return TraverseByPrefix(r, MakePrefix(codeHeader), func(key []byte, getValue func(destVal any) error) (bail bool, err error) {
		var h flow.Header
		err = getValue(&h)
		if err != nil {
			return true, err
		}
		// Skip headers with view beyond the archived threshold
		if h.View > ArchiveLatestFinalizedView {
			return false, nil
		}
		if filter(&h) {
			*found = append(*found, h)
		}
		return false, nil
	}, storage.DefaultIteratorOptions())

	// return TraverseByPrefix(r, MakePrefix(codeHeader), func(key []byte, getValue func(destVal any) error) (bail bool, err error) {
	// 	var h flow.Header
	// 	err = getValue(&h)
	// 	if err != nil {
	// 		return true, err
	// 	}
	// 	if filter(&h) {
	// 		*found = append(*found, h)
	// 	}
	// 	return false, nil
	// }, storage.DefaultIteratorOptions())
}
