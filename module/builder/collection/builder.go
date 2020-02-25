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

		// for each un-finalized ancestor of our new block, retrieve the list
		// of pending transactions; we use this to exclude transactions that
		// already exist in this fork.
		// TODO we need to check that we aren't duplicating payload items from FINALIZED blocks
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
			err = procedure.RetrieveClusterPayload(ancestor.PayloadHash, &payload)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve ancestor payload: %w", err)
			}

			// insert the transactions into the lookup
			for _, txHash := range payload.Collection.Transactions {
				txLookup[txHash] = struct{}{}
			}

			// continue with the next ancestor in the chain
			ancestorID = ancestor.ParentID
		}

		// STEP TWO: build a payload that includes as many transactions from the
		// memory pool as possible without including any that already exist on
		// our fork.
		// TODO make empty collections / limit size based on collection min/max size constraints
		var txIDs []flow.Identifier
		for _, flowTx := range b.transactions.All() {
			_, exists := txLookup[flowTx.ID()]
			if exists {
				continue
			}
			txIDs = append(txIDs, flowTx.ID())
		}

		// STEP THREE: we have a set of transactions that are valid to include
		// on this fork. Now we need to create the collection that will be
		// used in the payload, store and index it in storage, and insert the
		// header.

		// create and insert the collection
		// TODO when are individual transactions inserted to storage? may need to do that here
		collection := flow.LightCollection{Transactions: txIDs}
		err = operation.InsertCollection(&collection)(tx)
		if err != nil {
			return fmt.Errorf("could not insert collection: %w", err)
		}

		// build the payload
		payload := cluster.Payload{
			Collection: collection,
		}

		// index the payload by hash
		err = procedure.IndexClusterPayload(&payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index payload: %w", err)
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
			View:          0,
			ParentSigners: nil,
			ParentSigs:    nil,
			ProposerID:    flow.ZeroID,
			ProposerSig:   nil,
		}

		// set fields specific to the consensus algorithm
		setter(header)

		// insert the header for the newly built block
		err = operation.InsertHeader(header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	return header, nil
}
