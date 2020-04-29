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
//
// NOTE: Builder is NOT safe for use with multiple goroutines. Since the
// HotStuff event loop is the only consumer of this interface and is single
// threaded, this is OK.
type Builder struct {
	db           *badger.DB
	transactions mempool.Transactions
}

func NewBuilder(db *badger.DB, transactions mempool.Transactions, chainID string) *Builder {
	b := &Builder{
		db:           db,
		transactions: transactions,
	}
	return b
}

// BuildOn creates a new block built on the given parent. It produces a payload
// that is valid with respect to the un-finalized chain it extends.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header)) (*flow.Header, error) {
	var header *flow.Header
	err := b.db.Update(func(tx *badger.Txn) error {

		// retrieve the parent to set the height and have chain ID
		var parent flow.Header
		err := operation.RetrieveHeader(parentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		// retrieve the finalized head in order to know which transactions
		// have expired and should be discarded
		var boundary uint64
		err = operation.RetrieveBoundaryForCluster(parent.ChainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		var finalID flow.Identifier
		err = operation.RetrieveNumberForCluster(parent.ChainID, boundary, &finalID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized ID: %w", err)
		}

		var final flow.Header
		err = operation.RetrieveHeader(finalID, &final)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized header: %w", err)
		}

		// build a payload that includes as many transactions from the memory
		// that have not expired
		// TODO make empty collections / limit size based on collection min/max size constraints
		var candidateTxIDs []flow.Identifier
		for _, candidateTx := range b.transactions.All() {

			candidateID := candidateTx.ID()
			refID := candidateTx.ReferenceBlockID

			// find the transaction's reference block and ensure it has not expired
			// we haven't retrieved this yet, get it from storage first
			var ref flow.Header
			err = operation.RetrieveHeader(refID, &ref)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve reference block: %w", err)
			}

			// sanity check: ensure the reference block is from the main chain
			if ref.ChainID != flow.DefaultChainID {
				return fmt.Errorf("invalid reference block (chain_id=%s)", ref.ChainID)
			}

			// ensure the reference block is not too old, using a 10-block buffer
			if final.Height > ref.Height && final.Height-ref.Height > flow.DefaultTransactionExpiry-10 {
				// the transaction is expired, it will never be valid
				b.transactions.Rem(candidateID)
				continue
			}

			candidateTxIDs = append(candidateTxIDs, candidateTx.ID())
		}

		// find any transactions that conflict with finalized or un-finalized
		// blocks
		var invalidIDs map[flow.Identifier]struct{}
		err = operation.CheckCollectionPayload(parent.Height, parent.ID(), candidateTxIDs, &invalidIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not check collection payload: %w", err)
		}

		// populate the final list of transaction IDs for the block - these
		// are guaranteed to be valid
		var finalTransactions []*flow.TransactionBody
		for _, txID := range candidateTxIDs {

			_, isInvalid := invalidIDs[txID]
			if isInvalid {
				// remove from mempool, it will never be valid
				b.transactions.Rem(txID)
				continue
			}

			// add ONLY non-conflicting transaction to the final payload
			nextTx, err := b.transactions.ByID(txID)
			if err != nil {
				return fmt.Errorf("could not get transaction: %w", err)
			}
			finalTransactions = append(finalTransactions, nextTx)
		}

		// STEP THREE: we have a set of transactions that are valid to include
		// on this fork. Now we need to create the collection that will be
		// used in the payload, store and index it in storage, and insert the
		// header.

		// build the payload from the transactions
		payload := cluster.PayloadFromTransactions(finalTransactions...)

		header = &flow.Header{
			ChainID:     parent.ChainID,
			ParentID:    parentID,
			Height:      parent.Height + 1,
			PayloadHash: payload.Hash(),
			Timestamp:   time.Now().UTC(),

			// the following fields should be set by the provided setter function
			View:           0,
			ParentVoterIDs: nil,
			ParentVoterSig: nil,
			ProposerID:     flow.ZeroID,
			ProposerSig:    nil,
		}

		// set fields specific to the consensus algorithm
		setter(header)

		// insert the header for the newly built block
		err = operation.InsertHeader(header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert cluster header: %w", err)
		}

		// insert the payload
		// this inserts the collection AND all constituent transactions
		err = procedure.InsertClusterPayload(&payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert cluster payload: %w", err)
		}

		// index the payload by block ID
		err = procedure.IndexClusterPayload(header, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index cluster payload: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	return header, nil
}
