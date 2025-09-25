package collection

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/procedure"
)

// Finalizer is a simple wrapper around our temporary state to clean up after a
// block has been finalized. This involves removing the transactions within the
// finalized collection from the mempool and updating the finalized boundary in
// the cluster state.
type Finalizer struct {
	db           storage.DB
	lockManager  lockctx.Manager
	transactions mempool.Transactions
	pusher       collection.GuaranteedCollectionPublisher
	metrics      module.CollectionMetrics
}

// NewFinalizer creates a new finalizer for collection nodes.
func NewFinalizer(
	db storage.DB,
	lockManager lockctx.Manager,
	transactions mempool.Transactions,
	pusher collection.GuaranteedCollectionPublisher,
	metrics module.CollectionMetrics,
) *Finalizer {
	f := &Finalizer{
		db:           db,
		lockManager:  lockManager,
		transactions: transactions,
		pusher:       pusher,
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
func (f *Finalizer) MakeFinal(blockID flow.Identifier) error {
	// Acquire a lock for finalizing cluster blocks
	lctx := f.lockManager.NewContext()
	defer lctx.Release()
	if err := lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock); err != nil {
		return fmt.Errorf("could not acquire lock: %w", err)
	}

	reader := f.db.Reader()
	// retrieve the header of the block we want to finalize
	var header flow.Header
	err := operation.RetrieveHeader(reader, blockID, &header)
	if err != nil {
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// retrieve the current finalized cluster state boundary
	var boundary uint64
	err = operation.RetrieveClusterFinalizedHeight(reader, header.ChainID, &boundary)
	if err != nil {
		return fmt.Errorf("could not retrieve boundary: %w", err)
	}

	// retrieve the ID of the last finalized block as marker for stopping
	var headID flow.Identifier
	err = operation.LookupClusterBlockHeight(reader, header.ChainID, boundary, &headID)
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
		err = operation.RetrieveHeader(reader, parentID, &parent)
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
		err := f.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			clusterBlockID := steps[i].ID()

			// look up the transactions included in the payload
			step := steps[i]
			var payload cluster.Payload
			// This does not require a lock, as a block's payload once set never changes.
			err = procedure.RetrieveClusterPayload(rw.GlobalReader(), clusterBlockID, &payload)
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
			err = procedure.FinalizeClusterBlock(lctx, rw, clusterBlockID)
			if err != nil {
				return fmt.Errorf("could not finalize cluster block (id=%x): %w", clusterBlockID, err)
			}

			block, err := cluster.NewBlock(
				cluster.UntrustedBlock{
					HeaderBody: step.HeaderBody,
					Payload:    payload,
				},
			)
			if err != nil {
				return fmt.Errorf("could not build cluster block: %w", err)
			}

			f.metrics.ClusterBlockFinalized(block)

			// if the finalized collection is empty, we don't need to include it
			// in the reference height index or submit it to consensus nodes
			if len(payload.Collection.Transactions) == 0 {
				return nil
			}

			// look up the reference block height to populate index
			var refBlock flow.Header
			// This does not require a lock, as a block's header once set never changes.
			err = operation.RetrieveHeader(rw.GlobalReader(), payload.ReferenceBlockID, &refBlock)
			if err != nil {
				return fmt.Errorf("could not retrieve reference block (id=%x): %w", payload.ReferenceBlockID, err)
			}
			// index the finalized cluster block by reference block height
			err = operation.IndexClusterBlockByReferenceHeight(lctx, rw.Writer(), refBlock.Height, clusterBlockID)
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

			// TODO add real signatures here (https://github.com/onflow/flow-go-internal/issues/4569)
			// TODO: after adding real signature here add check for signature in NewCollectionGuarantee
			guarantee, err := flow.NewCollectionGuarantee(flow.UntrustedCollectionGuarantee{
				CollectionID:     payload.Collection.ID(),
				ReferenceBlockID: payload.ReferenceBlockID,
				ClusterChainID:   header.ChainID,
				SignerIndices:    step.ParentVoterIndices,
				Signature:        nil, // TODO: to remove because it's not easily verifiable by consensus nodes
			})
			if err != nil {
				return fmt.Errorf("could not construct guarantee: %w", err)
			}

			// collections should only be pushed to consensus nodes, once they are successfully persisted as finalized:
			storage.OnCommitSucceed(rw, func() {
				f.pusher.SubmitCollectionGuarantee((*messages.CollectionGuarantee)(guarantee))
			})

			return nil
		})
		if err != nil {
			return fmt.Errorf("could not finalize cluster block (%x): %w", steps[i].ID(), err)
		}
	}

	return nil
}
