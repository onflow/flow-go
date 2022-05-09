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

		// get the latest finalized block
		var final flow.Header
		err := procedure.RetrieveLatestFinalizedClusterHeader(chainID, &final)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized head: %w", err)
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
		for parentID != final.ID() {

			// get the parent of current block
			ancestor, err := m.headers.ByBlockID(parentID)
			if err != nil {
				return fmt.Errorf("could not get parent (%x): %w", block.Header.ParentID, err)
			}

			// if its height is below current boundary, the block does not connect
			// to the finalized protocol state and would break database consistency
			if ancestor.Height < final.Height {
				return state.NewOutdatedExtensionErrorf("block doesn't connect to finalized state. ancestor.Height (%v), final.Height (%v)",
					ancestor.Height, final.Height)
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

		// we go back a fixed number of  blocks to check payload for now
		// TODO look back based on reference block ID and expiry https://github.com/dapperlabs/flow-go/issues/3556
		limit := block.Header.Height - flow.DefaultTransactionExpiry
		if limit > block.Header.Height { // overflow check
			limit = 0
		}

		// check for duplicate transactions in block's ancestry
		txLookup := make(map[flow.Identifier]struct{})
		for _, tx := range block.Payload.Collection.Transactions {
			txLookup[tx.ID()] = struct{}{}
		}

		// first, check for duplicate transactions in the un-finalized ancestry
		duplicateTxIDs, err := m.checkDupeTransactionsInUnfinalizedAncestry(block, txLookup, final.Height)
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
	err = operation.RetryOnConflict(m.State.db.Update, procedure.InsertClusterBlock(block))
	if err != nil {
		return fmt.Errorf("could not insert cluster block: %w", err)
	}
	return nil
}

// checkDupeTransactionsInUnfinalizedAncestry checks for duplicate transactions in the un-finalized
// ancestry of the given block, and returns a list of all duplicates if there are any.
func (m *MutableState) checkDupeTransactionsInUnfinalizedAncestry(block *cluster.Block, includedTransactions map[flow.Identifier]struct{}, finalHeight uint64) ([]flow.Identifier, error) {
	var duplicateTxIDs []flow.Identifier

	ancestorID := block.Header.ParentID
	for {
		ancestor, err := m.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve ancestor header: %w", err)
		}

		if ancestor.Height <= finalHeight {
			break
		}

		payload, err := m.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve ancestor payload: %w", err)
		}

		for _, tx := range payload.Collection.Transactions {
			txID := tx.ID()
			_, duplicated := includedTransactions[txID]
			if duplicated {
				duplicateTxIDs = append(duplicateTxIDs, txID)
			}
		}
		ancestorID = ancestor.ParentID
	}

	return duplicateTxIDs, nil
}

// checkDupeTransactionsInFinalizedAncestry checks for duplicate transactions in the finalized
// ancestry, and returns a list of all duplicates if there are any.
func (m *MutableState) checkDupeTransactionsInFinalizedAncestry(includedTransactions map[flow.Identifier]struct{}, minRefHeight, maxRefHeight uint64) ([]flow.Identifier, error) {
	var duplicatedTxIDs []flow.Identifier

	// Let E be the global transaction expiry constant, measured in blocks
	//
	// 1. a cluster block's reference block height is equal to the lowest reference
	//    block height of all its constituent transactions
	// 2. a transaction with reference block height T is eligible for inclusion in
	//    any collection with reference block height C where C<=T<C+E
	// TODO: need to change anything to enforce the above? comment in builder?
	//
	// Therefore, to guarantee that we find all possible duplicates, we need to check
	// all collections with reference block height in the range (C-E,C+E)

	// the finalized cluster blocks which could possibly contain any conflicting transactions
	var clusterBlockIDs []flow.Identifier
	start := minRefHeight - flow.DefaultMaxCollectionByteSize + 1
	end := maxRefHeight
	err := m.db.View(operation.LookupClusterBlocksByReferenceHeightRange(start, end, &clusterBlockIDs))
	if err != nil {
		return nil, fmt.Errorf("could not lookup finalized cluster blocks by reference height range [%d,%d]: %w", start, end, err)
	}

	for _, blockID := range clusterBlockIDs {
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
