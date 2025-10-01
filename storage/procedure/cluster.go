package procedure

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

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
	//   2. When calling `operation.InsertHeader`, we append the storage operations for inserting the header to the
	//      provided write batch. Note that `operation.InsertHeader` checks whether the header already exists,
	//      returning [storage.ErrAlreadyExists] if so.
	//   3. We append all other storage indexing operations to the same write batch, without additional existence
	//      checks. This is safe, because this is the only place where these indexes are created, and we always
	//      store the block header first alongside the indices in one atomic batch. Hence, since we know from step 2
	//      that the header did not exist before, we also know that none of the other indexes existed before either
	//   4. We require that the caller holds the lock until the write batch has been committed.
	//      Thereby, we guarantee that no other thread can write data about the same block concurrently.
	// When these constraints are met, we know that no overwrites occurred because `InsertHeader`
	// includes guarantees that the key `blockID` has not yet been used before.
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) { // 1. check lock
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	// Here the key `blockID` is derived from the `block` via a collision-resistant hash function.
	// Hence, two different blocks having the same key is practically impossible.
	blockID := proposal.Block.ID()
	// 2. Store the block header; errors with [storage.ErrAlreadyExists] if some entry for `blockID` already exists
	err := operation.InsertHeader(lctx, rw, blockID, proposal.Block.ToHeader())
	if err != nil {
		return fmt.Errorf("could not insert cluster block header: %w", err)
	}

	// insert the block payload; without further overwrite checks (see above for explanation)
	err = operation.InsertProposalSignature(lctx, rw.Writer(), blockID, &proposal.ProposerSigData)
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
	err := operation.RetrieveHeader(r, blockID, &header)
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
	err := operation.RetrieveClusterFinalizedHeight(r, clusterID, &latestFinalizedHeight)
	if err != nil {
		return fmt.Errorf("could not retrieve latest finalized cluster block height: %w", err)
	}

	var finalID flow.Identifier
	err = operation.LookupClusterBlockHeight(r, clusterID, latestFinalizedHeight, &finalID)
	if err != nil {
		return fmt.Errorf("could not retrieve ID of latest finalized cluster block: %w", err)
	}

	err = operation.RetrieveHeader(r, finalID, final)
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
	err := operation.RetrieveHeader(r, blockID, &header)
	if err != nil {
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// get the chain ID, which determines which cluster state to query
	clusterID := header.ChainID

	// retrieve the latest finalized cluster block height
	var latestFinalizedHeight uint64
	err = operation.RetrieveClusterFinalizedHeight(r, clusterID, &latestFinalizedHeight)
	if err != nil {
		return fmt.Errorf("could not retrieve boundary: %w", err)
	}

	// retrieve the ID of the latest finalized cluster block
	var latestFinalizedBlockID flow.Identifier
	err = operation.LookupClusterBlockHeight(r, clusterID, latestFinalizedHeight, &latestFinalizedBlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve head: %w", err)
	}

	// sanity check: the previously latest finalized block is the parent of the block we are now finalizing
	if header.ParentID != latestFinalizedBlockID {
		return fmt.Errorf("can't finalize non-child of chain head")
	}

	// index the block by its height
	err = operation.IndexClusterBlockHeight(lctx, rw, clusterID, header.Height, blockID)
	if err != nil {
		return fmt.Errorf("could not index cluster block height: %w", err)
	}

	// update the finalized boundary
	err = operation.UpdateClusterFinalizedHeight(lctx, rw, clusterID, header.Height)
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
	err := operation.LookupCollectionPayload(rw.GlobalReader(), blockID, &txIDs)
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
	err = operation.UpsertCollection(writer, light) // collection is keyed by content hash, hence no overwrite protection is needed
	if err != nil {
		return fmt.Errorf("could not insert payload collection: %w", err)
	}

	// persist constituent transactions:
	for _, colTx := range payload.Collection.Transactions {
		err = operation.UpsertTransaction(writer, colTx.ID(), colTx) // as transaction is keyed by content hash, hence no overwrite protection is needed
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
	err = operation.IndexCollectionPayload(lctx, writer, blockID, txIDs)
	if err != nil {
		return fmt.Errorf("could not index collection: %w", err)
	}

	// insert the reference block ID
	err = operation.IndexReferenceBlockByClusterBlock(lctx, writer, blockID, payload.ReferenceBlockID)
	if err != nil {
		return fmt.Errorf("could not insert reference block ID: %w", err)
	}

	return nil
}

// RetrieveClusterPayload retrieves a cluster consensus block payload by block ID.
func RetrieveClusterPayload(r storage.Reader, blockID flow.Identifier, payload *cluster.Payload) error {
	// lookup the reference block ID
	var refID flow.Identifier
	err := operation.LookupReferenceBlockByClusterBlock(r, blockID, &refID)
	if err != nil {
		return fmt.Errorf("could not retrieve reference block ID: %w", err)
	}

	// lookup collection transaction IDs
	var txIDs []flow.Identifier
	err = operation.LookupCollectionPayload(r, blockID, &txIDs)
	if err != nil {
		return fmt.Errorf("could not look up collection payload: %w", err)
	}

	colTransactions := make([]*flow.TransactionBody, 0, len(txIDs))
	// retrieve individual transactions
	for _, txID := range txIDs {
		var nextTx flow.TransactionBody
		err = operation.RetrieveTransaction(r, txID, &nextTx)
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
