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

// InsertClusterBlock inserts a cluster consensus block, updating all
// associated indexes.
func InsertClusterBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, proposal *cluster.Proposal) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	// store the block header
	blockID := proposal.Block.ID()
	err := operation.InsertHeader(lctx, rw, blockID, proposal.Block.ToHeader())
	if err != nil {
		return fmt.Errorf("could not insert header: %w", err)
	}

	// since InsertHeader already checks for duplicates, we can safely
	// assume that the block header is new and there is no existing index
	// for other data related to this block ID.

	// insert the block payload
	err = InsertClusterPayload(lctx, rw.Writer(), blockID, &proposal.Block.Payload)
	if err != nil {
		return fmt.Errorf("could not insert payload: %w", err)
	}

	// index the child block for recovery
	err = InsertNewClusterBlock(lctx, rw, blockID, proposal.Block.ParentID)
	if err != nil {
		return fmt.Errorf("could not index new block: %w", err)
	}
	return nil
}

// RetrieveClusterBlock retrieves a cluster consensus block by block ID.
func RetrieveClusterBlock(r storage.Reader, blockID flow.Identifier, block *cluster.Block) error {
	// retrieve the block header
	var header flow.Header
	err := operation.RetrieveHeader(r, blockID, &header)
	if err != nil {
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// retrieve payload
	var payload cluster.Payload
	err = RetrieveClusterPayload(r, blockID, &payload)
	if err != nil {
		return fmt.Errorf("could not retrieve payload: %w", err)
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

// RetrieveLatestFinalizedClusterHeader retrieves the latest finalized for the
// given cluster chain ID.
func RetrieveLatestFinalizedClusterHeader(r storage.Reader, chainID flow.ChainID, final *flow.Header) error {
	var boundary uint64
	err := operation.RetrieveClusterFinalizedHeight(r, chainID, &boundary)
	if err != nil {
		return fmt.Errorf("could not retrieve boundary: %w", err)
	}

	var finalID flow.Identifier
	err = operation.LookupClusterBlockHeight(r, chainID, boundary, &finalID)
	if err != nil {
		return fmt.Errorf("could not retrieve final ID: %w", err)
	}

	err = operation.RetrieveHeader(r, finalID, final)
	if err != nil {
		return fmt.Errorf("could not retrieve finalized header: %w", err)
	}

	return nil
}

// FinalizeClusterBlock finalizes a block in cluster consensus.
func FinalizeClusterBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	r := rw.GlobalReader()
	writer := rw.Writer()
	// retrieve the header to check the parent
	var header flow.Header
	err := operation.RetrieveHeader(r, blockID, &header)
	if err != nil {
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// get the chain ID, which determines which cluster state to query
	chainID := header.ChainID

	// retrieve the current finalized state boundary
	var boundary uint64
	err = operation.RetrieveClusterFinalizedHeight(r, chainID, &boundary)
	if err != nil {
		return fmt.Errorf("could not retrieve boundary: %w", err)
	}

	// retrieve the ID of the boundary head
	var headID flow.Identifier
	err = operation.LookupClusterBlockHeight(r, chainID, boundary, &headID)
	if err != nil {
		return fmt.Errorf("could not retrieve head: %w", err)
	}

	// check that the head ID is the parent of the block we finalize
	if header.ParentID != headID {
		return fmt.Errorf("can't finalize non-child of chain head")
	}

	// index the block by its height
	err = operation.IndexClusterBlockHeight(lctx, writer, chainID, header.Height, blockID)
	if err != nil {
		return fmt.Errorf("could not index cluster block height: %w", err)
	}

	// update the finalized boundary
	err = operation.UpsertClusterFinalizedHeight(lctx, writer, chainID, header.Height)
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
func InsertClusterPayload(lctx lockctx.Proof, writer storage.Writer, blockID flow.Identifier, payload *cluster.Payload) error {
	if !lctx.HoldsLock(storage.LockInsertOrFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOrFinalizeClusterBlock)
	}

	// Only need to check if the lock is held, no need to check if is already stored,
	// because the duplication check is done when storing a header, which is in the same
	// batch update and holding the same lock.

	// Cluster payloads only contain a single collection
	// We allow duplicates, because it is valid for two competing forks to have the same payload.
	// Payloads are keyed by content hash, so duplicate values have the same key.
	light := payload.Collection.Light()
	err := operation.UpsertCollection(writer, light) // keyed by content hash so no lock needed
	if err != nil {
		return fmt.Errorf("could not insert payload collection: %w", err)
	}

	// insert constituent transactions
	for _, colTx := range payload.Collection.Transactions {
		err = operation.UpsertTransaction(writer, colTx.ID(), colTx) // keyed by content hash so no lock needed
		if err != nil {
			return fmt.Errorf("could not insert payload transaction: %w", err)
		}
	}

	// index the transaction IDs within the collection
	txIDs := payload.Collection.Light().Transactions
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
