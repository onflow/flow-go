package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// This file implements storage functions for chain state book-keeping of
// collection node cluster consensus. In contrast to the corresponding functions
// for regular consensus, these functions include the cluster ID in order to
// support storing multiple chains, for example during epoch switchover.

// IndexClusterBlockHeight indexes a cluster block ID by the cluster ID and block height.
// The function ensures data integrity by first checking if a block ID already exists for the given
// cluster and height, and rejecting overwrites with different values. This function is idempotent,
// i.e. repeated calls with the *initially* indexed value are no-ops.
//
// CAUTION:
//   - Confirming that no value is already stored and the subsequent write must be atomic to prevent data corruption.
//     The caller must acquire the [storage.LockInsertOrFinalizeClusterBlock] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrDataMismatch] if a *different* block ID is already indexed for the same cluster and height
func IndexClusterBlockHeight(lctx lockctx.Proof, rw storage.ReaderBatchWriter, clusterID flow.ChainID, height uint64, blockID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}

	key := MakePrefix(codeFinalizedCluster, clusterID, height)
	var existing flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		if existing != blockID {
			return fmt.Errorf("cluster block height already indexed with different block ID: %s vs %s: %w", existing, blockID, storage.ErrDataMismatch)
		}
		return nil // for the specified height, the finalized block is already set to `blockID`
	}
	// We do NOT want to continue with the WRITE UNLESS `storage.ErrNotFound` was received when checking for existing data.
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to check existing cluster block height index: %w", err)
	}

	return UpsertByKey(rw.Writer(), key, blockID)
}

// LookupClusterBlockHeight retrieves the ID of a finalized cluster block at the given height produced by the specified cluster.
// Note that only finalized cluster blocks are indexed by height to guarantee uniqueness.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no finalized block from the specified cluster is known at the given height
func LookupClusterBlockHeight(r storage.Reader, clusterID flow.ChainID, height uint64, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeFinalizedCluster, clusterID, height), blockID)
}

// BootstrapClusterFinalizedHeight initializes the latest finalized cluster block height for the given cluster.
//
// CAUTION:
//   - This function is intended to be called during bootstrapping only. It expects that the height of the latest
//     known finalized cluster block has not yet been persisted.
//   - Confirming that no value is already stored and the subsequent write must be atomic to prevent data corruption.
//     Therefore, the caller must acquire the [storage.LockInsertOrFinalizeClusterBlock] and hold it until the database
//     write has been committed.
//
// No error returns expected during normal operations.
func BootstrapClusterFinalizedHeight(lctx lockctx.Proof, rw storage.ReaderBatchWriter, clusterID flow.ChainID, number uint64) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}

	key := MakePrefix(codeClusterHeight, clusterID)

	var existing uint64
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		return fmt.Errorf("finalized height for cluster %v already initialized to %d", clusterID, existing)
	}

	// We do NOT want to continue with the WRITE UNLESS `storage.ErrNotFound` was received when checking for existing data.
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to check existing finalized height: %w", err)
	}

	return UpsertByKey(rw.Writer(), key, number)
}

// UpdateClusterFinalizedHeight updates (overwrites!) the latest finalized cluster block height for the given cluster.
//
// CAUTION:
//   - This function is intended for normal operations after bootstrapping. It expects that the height of the
//     latest known finalized cluster block has already been persisted. This function guarantees that the height is updated
//     sequentially, i.e. the new height is equal to the old height plus one. Otherwise, an exception is returned.
//   - Reading the current height value, checking that it increases sequentially, and writing the new value must happen in one
//     atomic operation to prevent data corruption. Hence, the caller must acquire [storage.LockInsertOrFinalizeClusterBlock]
//     and hold it until the database write has been committed.
//
// No error returns expected during normal operations.
func UpdateClusterFinalizedHeight(lctx lockctx.Proof, rw storage.ReaderBatchWriter, clusterID flow.ChainID, latestFinalizedHeight uint64) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}

	key := MakePrefix(codeClusterHeight, clusterID)

	var existing uint64
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err != nil {
		return fmt.Errorf("failed to check existing finalized height: %w", err)
	}

	if existing+1 != latestFinalizedHeight {
		return fmt.Errorf("finalization isn't sequential: existing %d, new %d", existing, latestFinalizedHeight)
	}

	return UpsertByKey(rw.Writer(), key, latestFinalizedHeight)
}

// RetrieveClusterFinalizedHeight retrieves the latest finalized cluster block height of the given cluster.
// For collector nodes in the specified cluster, this value should always exist (after bootstrapping).
// However, other nodes outside the cluster typically do not track the latest finalized heights for the
// different collector clusters.
//
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the latest finalized height for the specified cluster is not present in the database
func RetrieveClusterFinalizedHeight(r storage.Reader, clusterID flow.ChainID, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeClusterHeight, clusterID), height)
}

// IndexReferenceBlockByClusterBlock updates the reference block ID for the given
// cluster block ID. While each cluster block specifies a reference block in its
// payload, we maintain this additional lookup for performance reasons.
func IndexReferenceBlockByClusterBlock(lctx lockctx.Proof, w storage.Writer, clusterBlockID, refID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}

	// Only need to check if the lock is held, no need to check if is already stored,
	// because the duplication check is done when storing a header, which is in the same
	// batch update and holding the same lock.

	return UpsertByKey(w, MakePrefix(codeClusterBlockToRefBlock, clusterBlockID), refID)
}

// LookupReferenceBlockByClusterBlock looks up the reference block ID for the given
// cluster block ID. While each cluster block specifies a reference block in its
// payload, we maintain this additional lookup for performance reasons.
func LookupReferenceBlockByClusterBlock(r storage.Reader, clusterBlockID flow.Identifier, refID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeClusterBlockToRefBlock, clusterBlockID), refID)
}

// IndexClusterBlockByReferenceHeight indexes a cluster block ID by its reference
// block height. The cluster block ID is included in the key for more efficient
// traversal. Only finalized cluster blocks should be included in this index.
// The key looks like: <prefix 0:1><ref_height 1:9><cluster_block_id 9:41>
func IndexClusterBlockByReferenceHeight(lctx lockctx.Proof, w storage.Writer, refHeight uint64, clusterBlockID flow.Identifier) error {
	// Why is this lock necessary?
	// A single reference height can correspond to multiple cluster blocks. While we are finalizing blocks,
	// we may also be concurrently extending cluster blocks. This leads to simultaneous updates and reads
	// on keys sharing the same prefix. To prevent race conditions during these concurrent reads and writes,
	// synchronization is required when accessing these keys.
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}
	return UpsertByKey(w, MakePrefix(codeRefHeightToClusterBlock, refHeight, clusterBlockID), nil)
}

// LookupClusterBlocksByReferenceHeightRange traverses the ref_height->cluster_block
// index and returns any finalized cluster blocks which have a reference block with
// height in the given range. This is used to avoid including duplicate transaction
// when building or validating a new collection.
func LookupClusterBlocksByReferenceHeightRange(lctx lockctx.Proof, r storage.Reader, start, end uint64, clusterBlockIDs *[]flow.Identifier) error {
	// Why is this lock necessary?
	// A single reference height can correspond to multiple cluster blocks. While we are finalizing blocks,
	// we may also be concurrently extending cluster blocks. This leads to simultaneous updates and reads
	// on keys sharing the same prefix. To prevent race conditions during these concurrent reads and writes,
	// synchronization is required when accessing these keys.
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}
	startPrefix := MakePrefix(codeRefHeightToClusterBlock, start)
	endPrefix := MakePrefix(codeRefHeightToClusterBlock, end)
	prefixLen := len(startPrefix)
	checkFunc := func(key []byte) error {
		clusterBlockIDBytes := key[prefixLen:]
		var clusterBlockID flow.Identifier
		copy(clusterBlockID[:], clusterBlockIDBytes)
		*clusterBlockIDs = append(*clusterBlockIDs, clusterBlockID)

		// the info we need is stored in the key, never process the value
		return nil
	}

	return IterateKeysByPrefixRange(r, startPrefix, endPrefix, checkFunc)
}
