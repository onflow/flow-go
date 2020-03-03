package procedure

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// This file implements storage functions for blocks in cluster consensus.

// InsertClusterBlock inserts a cluster consensus block.
func InsertClusterBlock(block *cluster.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// check payload integrity
		if block.PayloadHash != block.Payload.Hash() {
			return fmt.Errorf("computed payload hash does not match header")
		}

		// store the block header
		err := operation.InsertHeader(&block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}

		// index the block payload
		err = IndexClusterPayload(&block.Header, &block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index payload: %w", err)
		}

		return nil
	}
}

// RetrieveClusterBlock retrieves a cluster consensus block by block ID.
func RetrieveClusterBlock(blockID flow.Identifier, block *cluster.Block) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// ensure block is empty
		*block = cluster.Block{}

		// retrieve the block header
		err := operation.RetrieveHeader(blockID, &block.Header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// retrieve payload
		err = RetrieveClusterPayload(block.ID(), &block.Payload)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve payload: %w", err)
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
		err = operation.RetrieveBoundaryForCluster(chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the ID of the boundary head
		var headID flow.Identifier
		err = operation.RetrieveNumberForCluster(chainID, boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// check that the head ID is the parent of the block we finalize
		if header.ParentID != headID {
			return fmt.Errorf("can't finalize non-child of chain head")
		}

		// insert block view -> ID mapping
		err = operation.InsertNumberForCluster(chainID, header.View, header.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert view->ID mapping: %w", err)
		}

		// update the finalized boundary
		err = operation.UpdateBoundaryForCluster(chainID, header.View)(tx)
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

// InsertClusterPayload inserts the payload for a cluster block.
func InsertClusterPayload(payload *cluster.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// cluster payloads only contain a single collection, allow duplicates,
		// because it is valid for two competing forks to have the same payload.
		err := operation.SkipDuplicates(operation.InsertCollection(&payload.Collection))(tx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		return nil
	}
}

// IndexClusterPayload indexes a cluster consensus block payload.
func IndexClusterPayload(header *flow.Header, payload *cluster.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// only index a collection if it exists
		var exists bool
		err := operation.CheckCollection(payload.Collection.ID(), &exists)(tx)
		if err != nil {
			return fmt.Errorf("could not check collection: %w", err)
		}

		if !exists {
			return fmt.Errorf("cannot index non-existent collection")
		}

		// index the transaction IDs within the collection
		err = operation.SkipDuplicates(operation.IndexCollectionPayload(header.Height, header.ID(), header.ParentID, &payload.Collection))(tx)
		if err != nil {
			return fmt.Errorf("could not index collection: %w", err)
		}

		return nil
	}
}

// RetrieveClusterPayload retrieves a cluster consensus block payload by block ID.
func RetrieveClusterPayload(blockID flow.Identifier, payload *cluster.Payload) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// ensure payload is empty
		*payload = cluster.Payload{}

		// retrieve the block header
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// lookup collection ID
		err = operation.LookupCollectionPayload(header.Height, blockID, header.ParentID, &payload.Collection)(tx)
		if err != nil {
			return fmt.Errorf("could not look up collection payload: %w", err)
		}

		return nil
	}
}
