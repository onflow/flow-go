package badger

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

type MutableState struct {
	*State
	tracer   module.Tracer
	headers  storage.Headers
	payloads storage.ClusterPayloads
}

func NewMutableState(state *State, tracer module.Tracer, headers storage.Headers, payloads storage.ClusterPayloads) (*MutableState, error) {
	mutableState := &MutableState{
		State:    state,
		tracer:   tracer,
		headers:  headers,
		payloads: payloads,
	}
	return mutableState, nil
}

// TODO (Ramtin) pass context here
func (m *MutableState) Extend(block *cluster.Block) error {

	blockID := block.ID()

	span, ctx, _ := m.tracer.StartCollectionSpan(context.Background(), blockID, trace.COLClusterStateMutatorExtend)
	defer span.Finish()

	err := m.State.db.View(func(tx *badger.Txn) error {

		setupSpan, _ := m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendSetup)

		header := block.Header
		payload := block.Payload

		// check chain ID
		if header.ChainID != m.State.clusterID {
			return state.NewInvalidExtensionErrorf("new block chain ID (%s) does not match configured (%s)", block.Header.ChainID, m.State.clusterID)
		}

		// check for a specified reference block
		// we also implicitly check this later, but can fail fast here
		if payload.ReferenceBlockID == flow.ZeroID {
			return state.NewInvalidExtensionError("new block has empty reference block ID")
		}

		// get the chain ID, which determines which cluster state to query
		chainID := header.ChainID

		// get the latest finalized cluster block and latest finalized consensus height
		var finalizedClusterBlock flow.Header
		err := procedure.RetrieveLatestFinalizedClusterHeader(chainID, &finalizedClusterBlock)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized cluster head: %w", err)
		}
		var finalizedConsensusHeight uint64
		err = operation.RetrieveFinalizedHeight(&finalizedConsensusHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized height on consensus chain: %w", err)
		}

		// get the header of the parent of the new block
		parent, err := m.headers.ByBlockID(header.ParentID)
		if err != nil {
			return fmt.Errorf("could not retrieve latest finalized header: %w", err)
		}

		// the extending block must increase height by 1 from parent
		if header.Height != parent.Height+1 {
			return state.NewInvalidExtensionErrorf("extending block height (%d) must be parent height + 1 (%d)",
				block.Header.Height, parent.Height)
		}

		// ensure that the extending block connects to the finalized state, we
		// do this by tracing back until we see a parent block that is the
		// latest finalized block, or reach height below the finalized boundary

		setupSpan.Finish()
		checkAnsSpan, _ := m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendCheckAncestry)

		// start with the extending block's parent
		parentID := header.ParentID
		for parentID != finalizedClusterBlock.ID() {

			// get the parent of current block
			ancestor, err := m.headers.ByBlockID(parentID)
			if err != nil {
				return fmt.Errorf("could not get parent (%x): %w", block.Header.ParentID, err)
			}

			// if its height is below current boundary, the block does not connect
			// to the finalized protocol state and would break database consistency
			if ancestor.Height < finalizedClusterBlock.Height {
				return state.NewOutdatedExtensionErrorf("block doesn't connect to finalized state. ancestor.Height (%d), final.Height (%d)",
					ancestor.Height, finalizedClusterBlock.Height)
			}

			parentID = ancestor.ParentID
		}

		checkAnsSpan.Finish()
		checkTxsSpan, _ := m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendCheckTransactionsValid)
		defer checkTxsSpan.Finish()

		// check that all transactions within the collection are valid
		minRefID := flow.ZeroID
		minRefHeight := uint64(math.MaxUint64)
		maxRefHeight := uint64(0)
		for _, flowTx := range payload.Collection.Transactions {
			refBlock, err := m.headers.ByBlockID(flowTx.ReferenceBlockID)
			if errors.Is(err, storage.ErrNotFound) {
				// unknown reference blocks are invalid
				return state.NewInvalidExtensionErrorf("unknown reference block (id=%x): %v", flowTx.ReferenceBlockID, err)
			}
			if err != nil {
				return fmt.Errorf("could not check reference block (id=%x): %w", flowTx.ReferenceBlockID, err)
			}

			if refBlock.Height < minRefHeight {
				minRefHeight = refBlock.Height
				minRefID = flowTx.ReferenceBlockID
			}
			if refBlock.Height > maxRefHeight {
				maxRefHeight = refBlock.Height
			}
		}

		// a valid collection must reference the oldest reference block among
		// its constituent transactions
		if payload.Collection.Len() > 0 && minRefID != payload.ReferenceBlockID {
			return state.NewInvalidExtensionErrorf(
				"reference block (id=%x) must match oldest transaction's reference block (id=%x)",
				payload.ReferenceBlockID, minRefID,
			)
		}
		// a valid collection must contain only transactions within its expiry window
		if payload.Collection.Len() > 0 {
			if maxRefHeight-minRefHeight >= flow.DefaultTransactionExpiry {
				return state.NewInvalidExtensionErrorf(
					"collection contains reference height range [%d,%d] exceeding expiry window size: %d",
					minRefHeight, maxRefHeight, flow.DefaultTransactionExpiry)
			}
		}

		// a valid collection must reference a valid reference block
		// NOTE: it is valid for a collection to be expired at this point,
		// otherwise we would compromise liveness of the cluster.
		refBlock, err := m.headers.ByBlockID(payload.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			return state.NewInvalidExtensionErrorf("unknown reference block (id=%x)", payload.ReferenceBlockID)
		}
		if err != nil {
			return fmt.Errorf("could not check reference block: %w", err)
		}

		// TODO ensure the reference block is part of the main chain
		_ = refBlock

		// check for duplicate transactions in block's ancestry
		txLookup := make(map[flow.Identifier]struct{})
		for _, tx := range block.Payload.Collection.Transactions {
			txID := tx.ID()
			if _, exists := txLookup[txID]; exists {
				return state.NewInvalidExtensionErrorf("collection contains transaction (id=%x) more than once", txID)
			}
			txLookup[txID] = struct{}{}
		}

		// first, check for duplicate transactions in the un-finalized ancestry
		duplicateTxIDs, err := m.checkDupeTransactionsInUnfinalizedAncestry(block, txLookup, finalizedClusterBlock.Height)
		if err != nil {
			return fmt.Errorf("could not check for duplicate txs in un-finalized ancestry: %w", err)
		}
		if len(duplicateTxIDs) > 0 {
			return state.NewInvalidExtensionErrorf("payload includes duplicate transactions in un-finalized ancestry (duplicates: %s)", duplicateTxIDs)
		}

		// second, check for duplicate transactions in the finalized ancestry
		duplicateTxIDs, err = m.checkDupeTransactionsInFinalizedAncestry(txLookup, minRefHeight, maxRefHeight)
		if err != nil {
			return fmt.Errorf("could not check for duplicate txs in finalized ancestry: %w", err)
		}
		if len(duplicateTxIDs) > 0 {
			return state.NewInvalidExtensionErrorf("payload includes duplicate transactions in finalized ancestry (duplicates: %s)", duplicateTxIDs)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not validate extending block: %w", err)
	}

	insertDbSpan, _ := m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendDBInsert)
	defer insertDbSpan.Finish()

	// insert the new block
	err = m.State.db.Update(procedure.InsertClusterBlock(block))
	if err != nil {
		return fmt.Errorf("could not insert cluster block: %w", err)
	}
	return nil
}

// checkDupeTransactionsInUnfinalizedAncestry checks for duplicate transactions in the un-finalized
// ancestry of the given block, and returns a list of all duplicates if there are any.
func (m *MutableState) checkDupeTransactionsInUnfinalizedAncestry(block *cluster.Block, includedTransactions map[flow.Identifier]struct{}, finalHeight uint64) ([]flow.Identifier, error) {

	var duplicateTxIDs []flow.Identifier
	err := fork.TraverseBackward(m.headers, block.Header.ParentID, func(ancestor *flow.Header) error {
		payload, err := m.payloads.ByBlockID(ancestor.ID())
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor payload: %w", err)
		}

		for _, tx := range payload.Collection.Transactions {
			txID := tx.ID()
			_, duplicated := includedTransactions[txID]
			if duplicated {
				duplicateTxIDs = append(duplicateTxIDs, txID)
			}
		}
		return nil
	}, fork.ExcludingHeight(finalHeight))

	return duplicateTxIDs, err
}

// checkDupeTransactionsInFinalizedAncestry checks for duplicate transactions in the finalized
// ancestry, and returns a list of all duplicates if there are any.
func (m *MutableState) checkDupeTransactionsInFinalizedAncestry(includedTransactions map[flow.Identifier]struct{}, minRefHeight, maxRefHeight uint64) ([]flow.Identifier, error) {
	var duplicatedTxIDs []flow.Identifier

	// Let E be the global transaction expiry constant, measured in blocks. For each
	// T ∈ `includedTransactions`, we have to decide whether the transaction
	// already appeared in _any_ finalized cluster block.
	// Notation:
	//   - consider a valid cluster block C and let c be its reference block height
	//   - consider a transaction T ∈ `includedTransactions` and let t denote its
	//     reference block height
	//
	// Boundary conditions:
	// 1. C's reference block height is equal to the lowest reference block height of
	//    all its constituent transactions. Hence, for collection C to potentially contain T, it must satisfy c <= t.
	// 2. For T to be eligible for inclusion in collection C, _none_ of the transactions within C are allowed
	// to be expired w.r.t. C's reference block. Hence, for collection C to potentially contain T, it must satisfy t < c + E.
	//
	// Therefore, for collection C to potentially contain transaction T, it must satisfy t - E < c <= t.
	// In other words, we only need to inspect collections with reference block height c ∈ (t-E, t].
	// Consequently, for a set of transactions, with `minRefHeight` (`maxRefHeight`) being the smallest (largest)
	// reference block height, we only need to inspect collections with c ∈ (minRefHeight-E, maxRefHeight].

	// the finalized cluster blocks which could possibly contain any conflicting transactions
	var clusterBlockIDs []flow.Identifier
	start := minRefHeight - flow.DefaultTransactionExpiry + 1
	if start > minRefHeight {
		start = 0 // overflow check
	}
	end := maxRefHeight
	err := m.db.View(operation.LookupClusterBlocksByReferenceHeightRange(start, end, &clusterBlockIDs))
	if err != nil {
		return nil, fmt.Errorf("could not lookup finalized cluster blocks by reference height range [%d,%d]: %w", start, end, err)
	}

	for _, blockID := range clusterBlockIDs {
		// TODO: could add LightByBlockID and retrieve only tx IDs
		payload, err := m.payloads.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve cluster payload (block_id=%x) to de-duplicate: %w", blockID, err)
		}
		for _, tx := range payload.Collection.Transactions {
			txID := tx.ID()
			_, duplicated := includedTransactions[txID]
			if duplicated {
				duplicatedTxIDs = append(duplicatedTxIDs, txID)
			}
		}
	}

	return duplicatedTxIDs, nil
}
