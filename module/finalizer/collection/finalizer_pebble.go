package collection

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
	"github.com/onflow/flow-go/storage/pebble/procedure"
)

// FinalizerPebble is a simple wrapper around our temporary state to clean up after a
// block has been finalized. This involves removing the transactions within the
// finalized collection from the mempool and updating the finalized boundary in
// the cluster state.
type FinalizerPebble struct {
	db           *pebble.DB
	transactions mempool.Transactions
	prov         network.Engine
	metrics      module.CollectionMetrics
}

// NewFinalizerPebble creates a new finalizer for collection nodes.
func NewFinalizerPebble(
	db *pebble.DB,
	transactions mempool.Transactions,
	prov network.Engine,
	metrics module.CollectionMetrics,
) *FinalizerPebble {
	f := &FinalizerPebble{
		db:           db,
		transactions: transactions,
		prov:         prov,
		metrics:      metrics,
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
// No errors are expected during normal operation.
func (f *FinalizerPebble) MakeFinal(blockID flow.Identifier) error {
	// retrieve the header of the block we want to finalize
	var header flow.Header
	err := operation.RetrieveHeader(blockID, &header)(f.db)
	if err != nil {
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// retrieve the current finalized cluster state boundary
	var boundary uint64
	err = operation.RetrieveClusterFinalizedHeight(header.ChainID, &boundary)(f.db)
	if err != nil {
		return fmt.Errorf("could not retrieve boundary: %w", err)
	}

	// retrieve the ID of the last finalized block as marker for stopping
	var headID flow.Identifier
	err = operation.LookupClusterBlockHeight(header.ChainID, boundary, &headID)(f.db)
	if err != nil {
		return fmt.Errorf("could not retrieve head: %w", err)
	}

	// there are no blocks to finalize, we may have already finalized
	// this block - exit early
	if boundary >= header.Height {
		return nil
	}

	// To finalize all blocks from the currently finalized one up to and
	// including the current, we first enumerate each of these blocks.
	// We start at the youngest block and remember all visited blocks,
	// while tracing back until we reach the finalized state
	steps := []*flow.Header{&header}
	parentID := header.ParentID
	for parentID != headID {
		var parent flow.Header
		err = operation.RetrieveHeader(parentID, &parent)(f.db)
		if err != nil {
			return fmt.Errorf("could not retrieve parent (%x): %w", parentID, err)
		}
		steps = append(steps, &parent)
		parentID = parent.ParentID
	}

	// now we can step backwards in order to go from oldest to youngest; for
	// each header, we reconstruct the block and then apply the related
	// changes to the protocol state
	// finalizing blocks one by one, each through a database batch update
	for i := len(steps) - 1; i >= 0; i-- {

		err := operation.WithReaderBatchWriter(f.db, func(tx storage.PebbleReaderBatchWriter) error {
			r, w := tx.ReaderWriter()

			clusterBlockID := steps[i].ID()

			// look up the transactions included in the payload
			step := steps[i]
			var payload cluster.Payload
			err = procedure.RetrieveClusterPayload(clusterBlockID, &payload)(r)
			if err != nil {
				return fmt.Errorf("could not retrieve payload for cluster block (id=%x): %w", clusterBlockID, err)
			}

			// remove the transactions from the memory pool
			for _, colTx := range payload.Collection.Transactions {
				txID := colTx.ID()
				// ignore result -- we don't care whether the transaction was in the pool
				_ = f.transactions.Remove(txID)
			}

			// finalize the block in cluster state
			err = procedure.FinalizeClusterBlock(clusterBlockID)(tx)
			if err != nil {
				return fmt.Errorf("could not finalize cluster block (id=%x): %w", clusterBlockID, err)
			}

			block := &cluster.Block{
				Header:  step,
				Payload: &payload,
			}
			f.metrics.ClusterBlockFinalized(block)

			// if the finalized collection is empty, we don't need to include it
			// in the reference height index or submit it to consensus nodes
			if len(payload.Collection.Transactions) == 0 {
				return nil
			}

			// look up the reference block height to populate index
			var refBlock flow.Header
			err = operation.RetrieveHeader(payload.ReferenceBlockID, &refBlock)(r)
			if err != nil {
				return fmt.Errorf("could not retrieve reference block (id=%x): %w", payload.ReferenceBlockID, err)
			}
			// index the finalized cluster block by reference block height
			err = operation.IndexClusterBlockByReferenceHeight(refBlock.Height, clusterBlockID)(w)
			if err != nil {
				return fmt.Errorf("could not index cluster block (id=%x) by reference height (%d): %w", clusterBlockID, refBlock.Height, err)
			}

			//TODO when we incorporate HotStuff AND require BFT, the consensus
			// node will need to be able ensure finalization by checking a
			// 3-chain of children for this block. Probably it will be simplest
			// to have a follower engine configured for the cluster chain
			// running on consensus nodes, rather than pushing finalized blocks
			// explicitly.
			// For now, we just use the parent signers as the guarantors of this
			// collection.

			// TODO add real signatures here (2711)
			f.prov.SubmitLocal(&messages.SubmitCollectionGuarantee{
				Guarantee: flow.CollectionGuarantee{
					CollectionID:     payload.Collection.ID(),
					ReferenceBlockID: payload.ReferenceBlockID,
					ChainID:          header.ChainID,
					SignerIndices:    step.ParentVoterIndices,
					Signature:        nil, // TODO: to remove because it's not easily verifiable by consensus nodes
				},
			})
			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}
