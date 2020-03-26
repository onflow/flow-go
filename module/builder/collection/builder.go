package collection

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Builder is the builder for collection block payloads. Upon providing a
// payload hash, it also memorizes the payload contents.
type Builder struct {
	db           *badger.DB
	transactions mempool.Transactions
	chainID      string
}

func NewBuilder(db *badger.DB, transactions mempool.Transactions, chainID string) *Builder {
	b := &Builder{
		db:           db,
		transactions: transactions,
		chainID:      chainID,
	}
	return b
}

// BuildOn creates a new block built on the given parent. It produces a payload
// that is valid with respect to the un-finalized chain it extends.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header)) (*flow.Header, error) {
	var header *flow.Header
	err := b.db.Update(func(tx *badger.Txn) error {

		// STEP ONE: get the payload contents that are included in ancestor
		// blocks which are not finalized yet; this allows us to avoid
		// including them in a block on the same fork twice.
		var boundary uint64
		err := operation.RetrieveBoundaryForCluster(b.chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		var finalizedID flow.Identifier
		err = operation.RetrieveNumberForCluster(b.chainID, boundary, &finalizedID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized ID: %w", err)
		}

		// for each un-finalized ancestor of our new block, retrieve the list
		// of pending transactions; we use this to exclude transactions that
		// already exist in this fork.
		ancestorID := parentID
		txLookup := make(map[flow.Identifier]struct{})
		for {

			// retrieve the header for the ancestor
			var ancestor flow.Header
			err = operation.RetrieveHeader(ancestorID, &ancestor)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve ancestor (id=%x): %w", ancestorID, err)
			}

			// if we have reached the finalized boundary, stop indexing
			if ancestor.View <= boundary {
				break
			}

			// look up the cluster payload (ie. the collection)
			var payload cluster.Payload
			err = procedure.RetrieveClusterPayload(ancestor.ID(), &payload)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve ancestor payload: %w", err)
			}

			// insert the transactions into the lookup
			for _, txID := range payload.Collection.Transactions {
				txLookup[txID] = struct{}{}
			}

			// continue with the next ancestor in the chain
			ancestorID = ancestor.ParentID
		}

		// STEP TWO: build a payload that includes as many transactions from the
		// memory pool as possible without including any that already exist on
		// our fork.
		// TODO make empty collections / limit size based on collection min/max size constraints
		var candidateTxIDs []flow.Identifier
		for _, flowTx := range b.transactions.All() {
			_, exists := txLookup[flowTx.ID()]
			if exists {
				continue
			}
			candidateTxIDs = append(candidateTxIDs, flowTx.ID())
		}

		// find any guarantees that conflict with FINALIZED blocks
		var invalidIDs map[flow.Identifier]struct{}
		err = operation.CheckCollectionPayload(boundary, finalizedID, candidateTxIDs, &invalidIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not check collection payload: %w", err)
		}

		// populate the final list of transaction IDs for the block - these
		// are guaranteed to be valid
		var finalTxIDs []flow.Identifier
		for _, txID := range candidateTxIDs {

			_, isInvalid := invalidIDs[txID]
			if isInvalid {
				// remove from mempool, it will never be valid
				b.transactions.Rem(txID)
				continue
			}

			// add ONLY non-conflicting transaction IDs to the final payload
			finalTxIDs = append(finalTxIDs, txID)
		}

		// STEP THREE: we have a set of transactions that are valid to include
		// on this fork. Now we need to create the collection that will be
		// used in the payload, store and index it in storage, and insert the
		// header.

		// insert transactions included in this collection
		for _, txID := range finalTxIDs {
			flowTx, err := b.transactions.ByID(txID)
			if err != nil {
				return fmt.Errorf("could not insert missing transaction: %w", err)
			}
			err = operation.SkipDuplicates(operation.InsertTransaction(flowTx))(tx)
			if err != nil {
				return fmt.Errorf("could not insert transaction: %w", err)
			}
		}

		// create and insert the collection
		collection := flow.LightCollection{Transactions: finalTxIDs}
		err = operation.SkipDuplicates(operation.InsertCollection(&collection))(tx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		// build the payload
		payload := cluster.Payload{
			Collection: collection,
		}

		// retrieve the parent to set the height
		var parent flow.Header
		err = operation.RetrieveHeader(parentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		header = &flow.Header{
			ChainID:     b.chainID,
			ParentID:    parentID,
			Height:      parent.Height + 1,
			PayloadHash: payload.Hash(),
			Timestamp:   time.Now().UTC(),

			// the following fields should be set by the provided setter function
			View:                    0,
			ParentSigners:           nil,
			ParentStakingSigs:       nil,
			ParentRandomBeaconSig:   nil,
			ProposerID:              flow.ZeroID,
			ProposerStakingSig:      nil,
			ProposerRandomBeaconSig: nil,
		}

		// set fields specific to the consensus algorithm
		setter(header)

		// insert the header for the newly built block
		err = operation.InsertHeader(header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}

		// index the payload by block ID
		err = procedure.IndexClusterPayload(header, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index payload: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	return header, nil
}
