package collection

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Finalizer is a simple wrapper around our temporary state to clean up after a
// block has been finalized. This involves removing the transactions within the
// finalized collection from the mempool and updating the finalized boundary in
// the cluster state.
type Finalizer struct {
	db           *badger.DB
	transactions mempool.Transactions
	prov         network.Engine
	metrics      module.Metrics
	chainID      string // aka cluster ID
}

// NewFinalizer creates a new finalizer for collection nodes.
func NewFinalizer(db *badger.DB, transactions mempool.Transactions, prov network.Engine, metrics module.Metrics, chainID string) *Finalizer {
	f := &Finalizer{
		db:           db,
		transactions: transactions,
		prov:         prov,
		metrics:      metrics,
		chainID:      chainID,
	}
	return f
}

// MakeFinal handles finalization logic for a block.
//
// The newly finalized block, and all un-finalized ancestors, are marked as
// finalized in the cluster state. All transactions included in the collections
// within the finalized blocks are removed from the mempool.
//
// This assumes that transactions are added to persistent state when they are
// included in a block proposal. Between entering the non-finalized chain state
// and being finalized, entities should be present in both the volatile memory
// pools and persistent storage.
func (f *Finalizer) MakeFinal(blockID flow.Identifier) error {
	return f.db.Update(func(tx *badger.Txn) error {

		// retrieve the header of the block we want to finalize
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// retrieve the current finalized cluster state boundary
		var boundary uint64
		err = operation.RetrieveBoundaryForCluster(f.chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the ID of the last finalized block as marker for stopping
		var headID flow.Identifier
		err = operation.RetrieveNumberForCluster(f.chainID, boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// there are no blocks to finalize, we may have already finalized
		// this block - exit early
		if boundary >= header.Height {
			return nil
		}

		// in order to validate the validity of all changes, we need to iterate
		// through the blocks that need to be finalized from oldest to youngest;
		// we thus start at the youngest remember all of the intermediary steps
		// while tracing back until we reach the finalized state
		steps := []*flow.Header{&header}
		parentID := header.ParentID
		for parentID != headID {
			var parent flow.Header
			err = operation.RetrieveHeader(parentID, &parent)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve parent (%x): %w", parentID, err)
			}
			steps = append(steps, &parent)
			parentID = parent.ParentID
		}

		// now we can step backwards in order to go from oldest to youngest; for
		// each header, we reconstruct the block and then apply the related
		// changes to the protocol state
		for i := len(steps) - 1; i >= 0; i-- {

			// look up the transactions included in the payload
			step := steps[i]
			var payload cluster.Payload
			err = procedure.RetrieveClusterPayload(step.ID(), &payload)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve cluster payload: %w", err)
			}

			// remove the transactions from the memory pool
			for _, txID := range payload.Collection.Transactions {
				ok := f.transactions.Rem(txID)
				if !ok {
					return fmt.Errorf("could not remove transaction from mempool (id=%x)", txID)
				}
				f.metrics.FinishTransactionToCollectionGuarantee(txID)
			}

			// finalize the block in cluster state
			err = procedure.FinalizeClusterBlock(step.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not finalize block: %w", err)
			}

			// don't bother submitting empty collections
			if payload.Collection.Len() == 0 {
				continue
			}

			// NOTE: when we incorporate HotStuff AND require BFT, the consensus
			// node will need to be able ensure finalization by checking a
			// 3-chain of children for this block. Probably it will be simplest
			// to have a follower engine configured for the cluster chain
			// running on consensus nodes, rather than pushing finalized blocks
			// explicitly.
			// For now, we just use the parent signers as the guarantors of this
			// collection.

			// TODO add real signatures here
			// https://github.com/dapperlabs/flow-go/issues/2711
			f.prov.SubmitLocal(&messages.SubmitCollectionGuarantee{
				Guarantee: flow.CollectionGuarantee{
					CollectionID: payload.Collection.ID(),
					SignerIDs:    step.ParentVoterIDs,
					Signature:    step.ParentVoterSig,
				},
			})
		}

		return nil
	})
}
