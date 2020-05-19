package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// This file implements storage functions for blocks in cluster consensus.

// InsertClusterBlock inserts a cluster consensus block, updating all
// associated indexes.
func InsertClusterBlock(block *cluster.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check payload integrity
		if block.Header.PayloadHash != block.Payload.Hash() {
			return fmt.Errorf("computed payload hash does not match header")
		}

		// store the block header
		blockID := block.ID()
		err := operation.InsertHeader(blockID, block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}

		// insert the block payload
		err = InsertClusterPayload(blockID, block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert payload: %w", err)
		}

		// start the new block with an empty child index
		err = operation.InsertBlockChildren(blockID, nil)(tx)
		if err != nil {
			return fmt.Errorf("could not create empty child index: %w", err)
		}

		return nil
	}
}

// RetrieveClusterBlock retrieves a cluster consensus block by block ID.
func RetrieveClusterBlock(blockID flow.Identifier, block *cluster.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the block header
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// retrieve payload
		var payload cluster.Payload
		err = RetrieveClusterPayload(blockID, &payload)(tx)
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
}

// RetrieveLatestFinalizedClusterHeader retrieves the latest finalized for the
// given cluster chain ID.
func RetrieveLatestFinalizedClusterHeader(chainID string, final *flow.Header) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		var boundary uint64
		err := operation.RetrieveClusterFinalizedHeight(chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		var finalID flow.Identifier
		err = operation.LookupClusterBlockHeight(chainID, boundary, &finalID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve final ID: %w", err)
		}

		err = operation.RetrieveHeader(finalID, final)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized header: %w", err)
		}

		return nil
	}
}

// FinalizeClusterBlock finalizes a block in cluster consensus.
func FinalizeClusterBlock(blockID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// retrieve the header to check the parent
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// get the chain ID, which determines which cluster state to query
		chainID := header.ChainID

		// retrieve the current finalized state boundary
		var boundary uint64
		err = operation.RetrieveClusterFinalizedHeight(chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the ID of the boundary head
		var headID flow.Identifier
		err = operation.LookupClusterBlockHeight(chainID, boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// check that the head ID is the parent of the block we finalize
		if header.ParentID != headID {
			return fmt.Errorf("can't finalize non-child of chain head")
		}

		// insert block view -> ID mapping
		err = operation.IndexClusterBlockHeight(chainID, header.Height, header.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert view->ID mapping: %w", err)
		}

		// update the finalized boundary
		err = operation.UpdateClusterFinalizedHeight(chainID, header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not update finalized boundary: %w", err)
		}

		// NOTE: we don't want to prune forks that have become invalid here, so
		// that we can keep validating entities and generating slashing
		// challenges for some time - the pruning should happen some place else
		// after a certain delay of blocks

		return nil
	}
}

// InsertClusterPayload inserts the payload for a cluster block. It inserts
// both the collection and all constituent transactions, allowing duplicates.
func InsertClusterPayload(blockID flow.Identifier, payload *cluster.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// cluster payloads only contain a single collection, allow duplicates,
		// because it is valid for two competing forks to have the same payload.
		light := payload.Collection.Light()
		err := operation.SkipDuplicates(operation.InsertCollection(&light))(tx)
		if err != nil {
			return fmt.Errorf("could not insert payload collection: %w", err)
		}

		// insert constituent transactions
		for _, colTx := range payload.Collection.Transactions {
			err = operation.SkipDuplicates(operation.InsertTransaction(colTx))(tx)
			if err != nil {
				return fmt.Errorf("could not insert payload transaction: %w", err)
			}
		}

		// index the transaction IDs within the collection
		txIDs := payload.Collection.Light().Transactions
		err = operation.SkipDuplicates(operation.IndexCollectionPayload(blockID, txIDs))(tx)
		if err != nil {
			return fmt.Errorf("could not index collection: %w", err)
		}

		// insert the reference block ID
		err = operation.IndexCollectionReference(blockID, payload.ReferenceBlockID)(tx)
		if err != nil {
			return fmt.Errorf("could not insert reference block ID: %w", err)
		}

		return nil
	}
}

// RetrieveClusterPayload retrieves a cluster consensus block payload by block ID.
func RetrieveClusterPayload(blockID flow.Identifier, payload *cluster.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// lookup the reference block ID
		var refID flow.Identifier
		err := operation.LookupCollectionReference(blockID, &refID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve reference block ID: %w", err)
		}

		// lookup collection transaction IDs
		var txIDs []flow.Identifier
		err = operation.LookupCollectionPayload(blockID, &txIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not look up collection payload: %w", err)
		}

		colTransactions := make([]*flow.TransactionBody, 0, len(txIDs))
		// retrieve individual transactions
		for _, txID := range txIDs {
			var nextTx flow.TransactionBody
			err = operation.RetrieveTransaction(txID, &nextTx)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve transaction: %w", err)
			}
			colTransactions = append(colTransactions, &nextTx)
		}

		*payload = cluster.PayloadFromTransactions(refID, colTransactions...)

		return nil
	}
}
