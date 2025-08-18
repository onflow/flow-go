package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// This file implements storage functions for chain state book-keeping of
// collection node cluster consensus. In contrast to the corresponding functions
// for regular consensus, these functions include the cluster ID in order to
// support storing multiple chains, for example during epoch switchover.

// IndexClusterBlockHeight UpsertByKeys a block number to block ID mapping for
// the given cluster.
func IndexClusterBlockHeight(lctx lockctx.Proof, w storage.Writer, clusterID flow.ChainID, height uint64, blockID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}

	return UpsertByKey(w, MakePrefix(codeFinalizedCluster, clusterID, height), blockID)
}

// LookupClusterBlockHeight retrieves a block ID by height for the given cluster
func LookupClusterBlockHeight(r storage.Reader, clusterID flow.ChainID, height uint64, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeFinalizedCluster, clusterID, height), blockID)
}

// UpsertByKeyClusterFinalizedHeight UpsertByKeys the finalized boundary for the given cluster.
func UpsertClusterFinalizedHeight(lctx lockctx.Proof, w storage.Writer, clusterID flow.ChainID, number uint64) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing lock: %v", storage.LockInsertOrFinalizeClusterBlock)
	}
	return UpsertByKey(w, MakePrefix(codeClusterHeight, clusterID), number)
}

// RetrieveClusterFinalizedHeight retrieves the finalized boundary for the given cluster.
func RetrieveClusterFinalizedHeight(r storage.Reader, clusterID flow.ChainID, number *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeClusterHeight, clusterID), number)
}

// IndexReferenceBlockByClusterBlock UpsertByKeys the reference block ID for the given
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
