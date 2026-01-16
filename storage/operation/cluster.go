package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/cluster"
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

// This file implements storage functions for blocks in cluster consensus.

// InsertClusterBlock inserts a cluster consensus block, updating all associated indexes.
//
// CAUTION:
//   - The caller must acquire the lock [storage.LockInsertOrFinalizeClusterBlock] and hold it
//     until the database write has been committed. This lock allows `InsertClusterBlock` to verify
//     that this block has not yet been indexed. In order to protect against accidental mutation
//     of existing data, this read and subsequent writes must be performed as one atomic operation.
//     Hence, the requirement to hold the lock until the write is committed.
//
// We return [storage.ErrAlreadyExists] if the block has already been persisted before, i.e. we only
// insert a block once. This error allows the caller to detect duplicate inserts.
// No other errors are expected during normal operation.
func InsertClusterBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, proposal *cluster.Proposal) error {
	// We need to enforce that each cluster block is inserted and indexed exactly once (no overwriting allowed):
	//   1. We check that the lock [storage.LockInsertOrFinalizeClusterBlock] for cluster block insertion is held.
	//   2. When calling `operation.InsertClusterHeader `, we append the storage operations for inserting the header to the
	//      provided write batch. Note that `operation.InsertClusterHeader` checks whether the header already exists,
	//      returning [storage.ErrAlreadyExists] if so.
	//   3. We append all other storage indexing operations to the same write batch, without additional existence
	//      checks. This is safe, because this is the only place where these indexes are created, and we always
	//      store the block header first alongside the indices in one atomic batch. Hence, since we know from step 2
	//      that the header did not exist before, we also know that none of the other indexes existed before either
	//   4. We require that the caller holds the lock until the write batch has been committed.
	//      Thereby, we guarantee that no other thread can write data about the same block concurrently.
	// When these constraints are met, we know that no overwrites occurred because `InsertClusterHeader`
	// includes guarantees that the key `blockID` has not yet been used before.
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) { // 1. check lock
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	// Here the key `blockID` is derived from the `block` via a collision-resistant hash function.
	// Hence, two different blocks having the same key is practically impossible.
	blockID := proposal.Block.ID()
	// 2. Store the block header; errors with [storage.ErrAlreadyExists] if some entry for `blockID` already exists
	err := InsertClusterHeader(lctx, rw, blockID, proposal.Block.ToHeader())
	if err != nil {
		return fmt.Errorf("could not insert cluster block header: %w", err)
	}

	// insert the block payload; without further overwrite checks (see above for explanation)
	err = InsertProposalSignature(lctx, rw.Writer(), blockID, &proposal.ProposerSigData)
	if err != nil {
		return fmt.Errorf("could not insert proposer signature: %w", err)
	}

	// insert the block payload
	err = InsertClusterPayload(lctx, rw, blockID, &proposal.Block.Payload)
	if err != nil {
		return fmt.Errorf("could not insert cluster block payload: %w", err)
	}

	// index the child block for recovery; without further overwrite checks (see above for explanation)
	err = IndexNewClusterBlock(lctx, rw, blockID, proposal.Block.ParentID)
	if err != nil {
		return fmt.Errorf("could not index new cluster block block: %w", err)
	}
	return nil
}

// RetrieveClusterBlock retrieves a cluster consensus block by block ID.
func RetrieveClusterBlock(r storage.Reader, blockID flow.Identifier, block *cluster.Block) error {
	// retrieve the block header
	var header flow.Header
	err := RetrieveHeader(r, blockID, &header)
	if err != nil {
		return fmt.Errorf("could not retrieve cluster block header: %w", err)
	}

	// retrieve payload
	var payload cluster.Payload
	err = RetrieveClusterPayload(r, blockID, &payload)
	if err != nil {
		return fmt.Errorf("could not retrieve cluster block payload: %w", err)
	}

	// overwrite block
	newBlock, err := cluster.NewBlock(
		cluster.UntrustedBlock{
			HeaderBody: header.HeaderBody,
			Payload:    payload,
		},
	)
	if err != nil {
		return fmt.Errorf("could not build cluster block: %w", err)
	}
	*block = *newBlock

	return nil
}

// RetrieveLatestFinalizedClusterHeader retrieves the latest finalized cluster block header from the specified cluster.
func RetrieveLatestFinalizedClusterHeader(r storage.Reader, clusterID flow.ChainID, final *flow.Header) error {
	var latestFinalizedHeight uint64
	err := RetrieveClusterFinalizedHeight(r, clusterID, &latestFinalizedHeight)
	if err != nil {
		return fmt.Errorf("could not retrieve latest finalized cluster block height: %w", err)
	}

	var finalID flow.Identifier
	err = LookupClusterBlockHeight(r, clusterID, latestFinalizedHeight, &finalID)
	if err != nil {
		return fmt.Errorf("could not retrieve ID of latest finalized cluster block: %w", err)
	}

	err = RetrieveHeader(r, finalID, final)
	if err != nil {
		return fmt.Errorf("could not retrieve header of latest finalized cluster block: %w", err)
	}
	return nil
}

// FinalizeClusterBlock finalizes a block in cluster consensus.
func FinalizeClusterBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	r := rw.GlobalReader()
	// retrieve the header to check the parent
	var header flow.Header
	err := RetrieveHeader(r, blockID, &header)
	if err != nil {
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// get the chain ID, which determines which cluster state to query
	clusterID := header.ChainID

	// retrieve the latest finalized cluster block height
	var latestFinalizedHeight uint64
	err = RetrieveClusterFinalizedHeight(r, clusterID, &latestFinalizedHeight)
	if err != nil {
		return fmt.Errorf("could not retrieve boundary: %w", err)
	}

	// retrieve the ID of the latest finalized cluster block
	var latestFinalizedBlockID flow.Identifier
	err = LookupClusterBlockHeight(r, clusterID, latestFinalizedHeight, &latestFinalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve head: %w", err)
	}

	// sanity check: the previously latest finalized block is the parent of the block we are now finalizing
	if header.ParentID != latestFinalizedBlockID {
		return fmt.Errorf("can't finalize non-child of chain head")
	}

	// index the block by its height
	err = IndexClusterBlockHeight(lctx, rw, clusterID, header.Height, blockID)
	if err != nil {
		return fmt.Errorf("could not index cluster block height: %w", err)
	}

	// update the finalized boundary
	err = UpdateClusterFinalizedHeight(lctx, rw, clusterID, header.Height)
	if err != nil {
		return fmt.Errorf("could not update finalized boundary: %w", err)
	}

	// NOTE: we don't want to prune forks that have become invalid here, so
	// that we can keep validating entities and generating slashing
	// challenges for some time - the pruning should happen some place else
	// after a certain delay of blocks

	return nil
}

// InsertClusterPayload inserts the payload for a cluster block. It inserts
// both the collection and all constituent transactions, allowing duplicates.
func InsertClusterPayload(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, payload *cluster.Payload) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	var txIDs []flow.Identifier
	err := LookupCollectionPayload(rw.GlobalReader(), blockID, &txIDs)
	if err == nil {
		return fmt.Errorf("collection payload already exists for block %s: %w", blockID, storage.ErrAlreadyExists)
	}
	if err != storage.ErrNotFound {
		return fmt.Errorf("unexpected error while attempting to retrieve collection payload: %w", err)
	}

	// STEP 1: persist the collection and constituent transactions.
	// A cluster payload essentially represents a single collection (batch of transactions) plus some auxilluary
	// information. Storing the collection on its own allows us to also retrieve it independently of the cluster
	// block's payload. We expect repeated requests to persist the same collection data here, because it is valid
	// to propose the same collection in two competing forks. However, we don't have to worry about repeated calls,
	// because collections and transactions are keyed by their respective content hashes. So a different value
	// should produce a different key, making accidental overwrites with inconsistent values impossible.
	// Here, we persist a reduced representation of the collection, only listing the constituent transactions by their hashes.
	light := payload.Collection.Light()
	writer := rw.Writer()
	err = UpsertCollection(writer, light) // collection is keyed by content hash, hence no overwrite protection is needed
	if err != nil {
		return fmt.Errorf("could not insert payload collection: %w", err)
	}

	// persist constituent transactions:
	for _, colTx := range payload.Collection.Transactions {
		err = UpsertTransaction(writer, colTx.ID(), colTx) // as transaction is keyed by content hash, hence no overwrite protection is needed
		if err != nil {
			return fmt.Errorf("could not insert payload transaction: %w", err)
		}
	}

	// STEP 2: for the cluster block ID, index the consistent transactions plus the auxilluary data from the playload.
	// Caution: Here we use the cluster block's ID as key, which is *not* uniquely determined by the indexed data.
	// Hence, we must ensure that we are not accidentally overwriting existing data (in case of a bug in the calling
	// code) with different values. This is ensured by the initial check confirming that the collection payload
	// has not yet been indexed (and the assumption that `IndexReferenceBlockByClusterBlock` is called nowhere else).
	txIDs = light.Transactions
	err = IndexCollectionPayload(lctx, writer, blockID, txIDs)
	if err != nil {
		return fmt.Errorf("could not index collection: %w", err)
	}

	// insert the reference block ID
	err = IndexReferenceBlockByClusterBlock(lctx, writer, blockID, payload.ReferenceBlockID)
	if err != nil {
		return fmt.Errorf("could not insert reference block ID: %w", err)
	}

	return nil
}

// RetrieveClusterPayload retrieves a cluster consensus block payload by block ID.
func RetrieveClusterPayload(r storage.Reader, blockID flow.Identifier, payload *cluster.Payload) error {
	// lookup the reference block ID
	var refID flow.Identifier
	err := LookupReferenceBlockByClusterBlock(r, blockID, &refID)
	if err != nil {
		return fmt.Errorf("could not retrieve reference block ID: %w", err)
	}

	// lookup collection transaction IDs
	var txIDs []flow.Identifier
	err = LookupCollectionPayload(r, blockID, &txIDs)
	if err != nil {
		return fmt.Errorf("could not look up collection payload: %w", err)
	}

	colTransactions := make([]*flow.TransactionBody, 0, len(txIDs))
	// retrieve individual transactions
	for _, txID := range txIDs {
		var nextTx flow.TransactionBody
		err = RetrieveTransaction(r, txID, &nextTx)
		if err != nil {
			return fmt.Errorf("could not retrieve transaction: %w", err)
		}
		colTransactions = append(colTransactions, &nextTx)
	}

	collection, err := flow.NewCollection(flow.UntrustedCollection{Transactions: colTransactions})
	if err != nil {
		return fmt.Errorf("could not build the collection from the transactions: %w", err)
	}
	newPayload, err := cluster.NewPayload(
		cluster.UntrustedPayload{
			ReferenceBlockID: refID,
			Collection:       *collection,
		},
	)
	if err != nil {
		return fmt.Errorf("could not build the payload: %w", err)
	}
	*payload = *newPayload

	return nil
}
