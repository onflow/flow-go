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
func InsertClusterBlock(lctx lockctx.Proof, rw storage.ReaderBatchWriter, block *cluster.Block) error {
	if !lctx.HoldsLock(storage.LockInsertClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	// check payload integrity
	if block.Header.PayloadHash != block.Payload.Hash() {
		return fmt.Errorf("computed payload hash does not match header")
	}

	// store the block header
	blockID := block.ID()
	err := operation.InsertHeader(rw.Writer(), blockID, block.Header)
	if err != nil {
		return fmt.Errorf("could not insert header: %w", err)
	}

	// insert the block payload
	err = InsertClusterPayload(lctx, rw, blockID, block.Payload)
	if err != nil {
		return fmt.Errorf("could not insert payload: %w", err)
	}

	// index the child block for recovery
	err = InsertNewClusterBlock(lctx, rw, blockID, block.Header.ParentID)
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
	*block = cluster.Block{
		Header:  &header,
		Payload: &payload,
	}

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
	if !lctx.HoldsLock(storage.LockFinalizeClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockFinalizeClusterBlock)
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
	err = operation.IndexClusterBlockHeight(lctx, writer, chainID, header.Height, header.ID())
	if err != nil {
		return fmt.Errorf("could not index cluster block height: %w", err)
	}

	// update the finalized boundary
	err = operation.UpsertClusterFinalizedHeight(writer, chainID, header.Height)
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
	if !lctx.HoldsLock(storage.LockInsertClusterBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertClusterBlock)
	}

	var txIDs []flow.Identifier
	err := operation.LookupCollectionPayload(rw.GlobalReader(), blockID, &txIDs)
	if err == nil {
		return fmt.Errorf("collection payload already exists for block %s: %w", blockID, storage.ErrAlreadyExists)
	}

	if err != storage.ErrNotFound {
		return fmt.Errorf("could not look up collection payload: %w", err)
	}

	// cluster payloads only contain a single collection, allow duplicates,
	// because it is valid for two competing forks to have the same payload.
	light := payload.Collection.Light()
	// SkipDuplicates here is to ignore the error if the collection already exists
	// This means the Insert operation is actually a Upsert operation.
	// The upsert is ok, because the data is unique by its ID
	writer := rw.Writer()
	err = operation.UpsertCollection(writer, &light)
	if err != nil {
		return fmt.Errorf("could not insert payload collection: %w", err)
	}

	// insert constituent transactions
	for _, colTx := range payload.Collection.Transactions {
		// SkipDuplicates here is to ignore the error if the collection already exists
		// This means the Insert operation is actually a Upsert operation.
		// The upsert is ok, because the data is unique by its ID
		err = operation.UpsertTransaction(writer, colTx.ID(), colTx)
		if err != nil {
			return fmt.Errorf("could not insert payload transaction: %w", err)
		}
	}

	// index the transaction IDs within the collection
	txIDs = payload.Collection.Light().Transactions
	// SkipDuplicates here is to ignore the error if the collection already exists
	// This means the Insert operation is actually a Upsert operation.
	err = operation.IndexCollectionPayload(writer, blockID, txIDs)
	if err != nil {
		return fmt.Errorf("could not index collection: %w", err)
	}

	// insert the reference block ID
	err = operation.IndexReferenceBlockByClusterBlock(writer, blockID, payload.ReferenceBlockID)
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

	*payload = cluster.PayloadFromTransactions(refID, colTransactions...)

	return nil
}
