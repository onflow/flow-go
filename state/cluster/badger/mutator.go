package badger

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	clusterstate "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/fork"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/procedure"
)

type MutableState struct {
	*State
	lockManager lockctx.Manager
	tracer      module.Tracer
	headers     storage.Headers
	payloads    storage.ClusterPayloads
}

var _ clusterstate.MutableState = (*MutableState)(nil)

func NewMutableState(state *State, lockManager lockctx.Manager, tracer module.Tracer, headers storage.Headers, payloads storage.ClusterPayloads) (*MutableState, error) {
	mutableState := &MutableState{
		State:       state,
		lockManager: lockManager,
		tracer:      tracer,
		headers:     headers,
		payloads:    payloads,
	}
	return mutableState, nil
}

// extendContext encapsulates all state information required in order to validate a candidate cluster block.
type extendContext struct {
	candidate                *cluster.Block // the proposed candidate cluster block
	finalizedClusterBlock    *flow.Header   // the latest finalized cluster block
	finalizedConsensusHeight uint64         // the latest finalized height on the main chain
	epochFirstHeight         uint64         // the first height of this cluster's operating epoch
	epochLastHeight          uint64         // the last height of this cluster's operating epoch (may be unknown)
	epochHasEnded            bool           // whether this cluster's operating epoch has ended (whether the above field is known)
}

// getExtendCtx reads all required information from the database in order to validate
// a candidate cluster block.
// No errors are expected during normal operation.
func (m *MutableState) getExtendCtx(candidate *cluster.Block) (extendContext, error) {
	var ctx extendContext
	ctx.candidate = candidate

	r := m.State.db.Reader()
	// get the latest finalized cluster block and latest finalized consensus height
	ctx.finalizedClusterBlock = new(flow.Header)
	err := procedure.RetrieveLatestFinalizedClusterHeader(r, candidate.ChainID, ctx.finalizedClusterBlock)
	if err != nil {
		return extendContext{}, fmt.Errorf("could not retrieve finalized cluster head: %w", err)
	}
	err = operation.RetrieveFinalizedHeight(r, &ctx.finalizedConsensusHeight)
	if err != nil {
		return extendContext{}, fmt.Errorf("could not retrieve finalized height on consensus chain: %w", err)
	}

	err = operation.RetrieveEpochFirstHeight(r, m.State.epoch, &ctx.epochFirstHeight)
	if err != nil {
		return extendContext{}, fmt.Errorf("could not get operating epoch first height: %w", err)
	}
	err = operation.RetrieveEpochLastHeight(r, m.State.epoch, &ctx.epochLastHeight)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			ctx.epochHasEnded = false
			return ctx, nil
		}
		return extendContext{}, fmt.Errorf("unexpected failure to retrieve final height of operating epoch: %w", err)
	}
	ctx.epochHasEnded = true

	return ctx, nil
}

// Extend introduces the given block into the cluster state as a pending
// without modifying the current finalized state.
// The block's parent must have already been successfully inserted.
// TODO(ramtin) pass context here
// Expected errors during normal operations:
//   - state.OutdatedExtensionError if the candidate block is outdated (e.g. orphaned)
//   - state.UnverifiableExtensionError if the reference block is _not_ a known finalized block
//   - state.InvalidExtensionError if the candidate block is invalid
func (m *MutableState) Extend(proposal *cluster.Proposal) error {
	candidate := proposal.Block
	parentSpan, ctx := m.tracer.StartCollectionSpan(context.Background(), candidate.ID(), trace.COLClusterStateMutatorExtend)
	defer parentSpan.End()

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendCheckHeader)
	err := m.checkHeaderValidity(&candidate)
	span.End()
	if err != nil {
		return fmt.Errorf("error checking header validity: %w", err)
	}

	span, _ = m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendGetExtendCtx)
	extendCtx, err := m.getExtendCtx(&candidate)
	span.End()
	if err != nil {
		return fmt.Errorf("error gettting extend context data: %w", err)
	}

	span, _ = m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendCheckAncestry)
	err = m.checkConnectsToFinalizedState(extendCtx)
	span.End()
	if err != nil {
		return fmt.Errorf("error checking connection to finalized state: %w", err)
	}

	span, _ = m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendCheckReferenceBlock)
	err = m.checkPayloadReferenceBlock(extendCtx)
	span.End()
	if err != nil {
		return fmt.Errorf("error checking reference block: %w", err)
	}

	lctx := m.lockManager.NewContext()
	err = lctx.AcquireLock(storage.LockInsertOrFinalizeClusterBlock)
	if err != nil {
		return fmt.Errorf("could not acquire lock for inserting cluster block: %w", err)
	}
	defer lctx.Release()

	span, _ = m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendCheckTransactionsValid)
	err = m.checkPayloadTransactions(lctx, extendCtx)
	span.End()
	if err != nil {
		return fmt.Errorf("error checking payload transactions: %w", err)
	}

	span, _ = m.tracer.StartSpanFromContext(ctx, trace.COLClusterStateMutatorExtendDBInsert)
	err = m.State.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return procedure.InsertClusterBlock(lctx, rw, proposal)
	})
	span.End()
	if err != nil {
		return fmt.Errorf("could not insert cluster block: %w", err)
	}

	return nil
}

// checkHeaderValidity validates that the candidate block has a header which is
// valid generally for inclusion in the cluster consensus, and w.r.t. its parent.
// Expected error returns:
//   - state.InvalidExtensionError if the candidate header is invalid
func (m *MutableState) checkHeaderValidity(candidate *cluster.Block) error {
	// check chain ID
	if candidate.ChainID != m.State.clusterID {
		return state.NewInvalidExtensionErrorf("new block chain ID (%s) does not match configured (%s)", candidate.ChainID, m.State.clusterID)
	}

	// get the header of the parent of the new block
	parent, err := m.headers.ByBlockID(candidate.ParentID)
	if err != nil {
		return irrecoverable.NewExceptionf("could not retrieve latest finalized header: %w", err)
	}

	// extending block must have correct parent view
	if candidate.ParentView != parent.View {
		return state.NewInvalidExtensionErrorf("candidate build with inconsistent parent view (candidate: %d, parent %d)",
			candidate.ParentView, parent.View)
	}

	// the extending block must increase height by 1 from parent
	if candidate.Height != parent.Height+1 {
		return state.NewInvalidExtensionErrorf("extending block height (%d) must be parent height + 1 (%d)",
			candidate.Height, parent.Height)
	}
	return nil
}

// checkConnectsToFinalizedState validates that the candidate block connects to
// the latest finalized state (ie. is not extending an orphaned fork).
// Expected error returns:
//   - state.OutdatedExtensionError if the candidate extends an orphaned fork
func (m *MutableState) checkConnectsToFinalizedState(ctx extendContext) error {
	parentID := ctx.candidate.ParentID
	finalizedID := ctx.finalizedClusterBlock.ID()
	finalizedHeight := ctx.finalizedClusterBlock.Height

	// start with the extending block's parent
	for parentID != finalizedID {
		// get the parent of current block
		ancestor, err := m.headers.ByBlockID(parentID)
		if err != nil {
			return irrecoverable.NewExceptionf("could not get parent which must be known (%x): %w", parentID, err)
		}

		// if its height is below current boundary, the block does not connect
		// to the finalized protocol state and would break database consistency
		if ancestor.Height < finalizedHeight {
			return state.NewOutdatedExtensionErrorf(
				"block doesn't connect to latest finalized block (height=%d, id=%x): orphaned ancestor (height=%d, id=%x)",
				finalizedHeight, finalizedID, ancestor.Height, parentID)
		}
		parentID = ancestor.ParentID
	}
	return nil
}

// checkPayloadReferenceBlock validates the reference block is valid.
//   - it must be a known, finalized block on the main consensus chain
//   - it must be within the cluster's operating epoch
//
// Expected error returns:
//   - state.InvalidExtensionError if the reference block is invalid for use.
//   - state.UnverifiableExtensionError if the reference block is unknown.
func (m *MutableState) checkPayloadReferenceBlock(ctx extendContext) error {
	payload := ctx.candidate.Payload

	// 1 - the reference block must be known
	refBlock, err := m.headers.ByBlockID(payload.ReferenceBlockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return state.NewUnverifiableExtensionError("cluster block references unknown reference block (id=%x)", payload.ReferenceBlockID)
		}
		return fmt.Errorf("could not check reference block: %w", err)
	}

	// 2 - the reference block must be finalized
	if refBlock.Height > ctx.finalizedConsensusHeight {
		// a reference block which is above the finalized boundary can't be verified yet
		return state.NewUnverifiableExtensionError("reference block is above finalized boundary (%d>%d)", refBlock.Height, ctx.finalizedConsensusHeight)
	} else {
		storedBlockIDForHeight, err := m.headers.BlockIDByHeight(refBlock.Height)
		if err != nil {
			return irrecoverable.NewExceptionf("could not look up block ID for finalized height: %w", err)
		}
		// a reference block with height at or below the finalized boundary must have been finalized
		if storedBlockIDForHeight != payload.ReferenceBlockID {
			return state.NewInvalidExtensionErrorf("cluster block references orphaned reference block (id=%x, height=%d), the block finalized at this height is %x",
				payload.ReferenceBlockID, refBlock.Height, storedBlockIDForHeight)
		}
	}

	// TODO ensure the reference block is part of the main chain https://github.com/onflow/flow-go/issues/4204
	_ = refBlock

	// 3 - the reference block must be within the cluster's operating epoch
	if refBlock.Height < ctx.epochFirstHeight {
		return state.NewInvalidExtensionErrorf("invalid reference block is before operating epoch for cluster, height %d<%d", refBlock.Height, ctx.epochFirstHeight)
	}
	if ctx.epochHasEnded && refBlock.Height > ctx.epochLastHeight {
		return state.NewInvalidExtensionErrorf("invalid reference block is after operating epoch for cluster, height %d>%d", refBlock.Height, ctx.epochLastHeight)
	}
	return nil
}

// checkPayloadTransactions validates the transactions included int the candidate cluster block's payload.
// It enforces:
//   - transactions are individually valid
//   - no duplicate transaction exists along the fork being extended
//   - the collection's reference block is equal to the oldest reference block among
//     its constituent transactions
//
// Expected error returns:
//   - state.InvalidExtensionError if the reference block is invalid for use.
//   - state.UnverifiableExtensionError if the reference block is unknown.
func (m *MutableState) checkPayloadTransactions(lctx lockctx.Proof, ctx extendContext) error {
	block := ctx.candidate
	payload := block.Payload

	if payload.Collection.Len() == 0 {
		return nil
	}

	// check that all transactions within the collection are valid
	// keep track of the min/max reference blocks - the collection must be non-empty
	// at this point so these are guaranteed to be set correctly
	minRefID := flow.ZeroID
	minRefHeight := uint64(math.MaxUint64)
	maxRefHeight := uint64(0)
	for _, flowTx := range payload.Collection.Transactions {
		refBlock, err := m.headers.ByBlockID(flowTx.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			// unknown reference blocks are invalid
			return state.NewUnverifiableExtensionError("collection contains tx (tx_id=%x) with unknown reference block (block_id=%x): %w", flowTx.ID(), flowTx.ReferenceBlockID, err)
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
	if minRefID != payload.ReferenceBlockID {
		return state.NewInvalidExtensionErrorf(
			"reference block (id=%x) must match oldest transaction's reference block (id=%x)",
			payload.ReferenceBlockID, minRefID,
		)
	}
	// a valid collection must contain only transactions within its expiry window
	if maxRefHeight-minRefHeight >= flow.DefaultTransactionExpiry {
		return state.NewInvalidExtensionErrorf(
			"collection contains reference height range [%d,%d] exceeding expiry window size: %d",
			minRefHeight, maxRefHeight, flow.DefaultTransactionExpiry)
	}

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
	duplicateTxIDs, err := m.checkDupeTransactionsInUnfinalizedAncestry(block, txLookup, ctx.finalizedClusterBlock.Height)
	if err != nil {
		return fmt.Errorf("could not check for duplicate txs in un-finalized ancestry: %w", err)
	}
	if len(duplicateTxIDs) > 0 {
		return state.NewInvalidExtensionErrorf("payload includes duplicate transactions in un-finalized ancestry (duplicates: %s)", duplicateTxIDs)
	}

	// second, check for duplicate transactions in the finalized ancestry
	// CAUTION: Finalization might progress while we are running this logic. However, finalization is not guaranteed to
	// follow the same fork as the one we are extending here. Hence, we might apply the transaction de-duplication logic
	// against blocks that do not belong to our fork. If we erroneously find a duplicated transaction, based on a block
	// that is not part of our fork, we would be raising an invalid slashing challenge, which would get this node slashed.
	duplicateTxIDs, err = m.checkDupeTransactionsInFinalizedAncestry(lctx, txLookup, minRefHeight, maxRefHeight, ctx.finalizedClusterBlock.Height)
	if err != nil {
		return fmt.Errorf("could not check for duplicate txs in finalized ancestry: %w", err)
	}
	if len(duplicateTxIDs) > 0 {
		return state.NewInvalidExtensionErrorf("payload includes duplicate transactions in finalized ancestry (duplicates: %s)", duplicateTxIDs)
	}

	return nil
}

// checkDupeTransactionsInUnfinalizedAncestry checks for duplicate transactions in the un-finalized
// ancestry of the given block, and returns a list of all duplicates if there are any.
// No errors are expected during normal operation.
func (m *MutableState) checkDupeTransactionsInUnfinalizedAncestry(block *cluster.Block, includedTransactions map[flow.Identifier]struct{}, finalHeight uint64) ([]flow.Identifier, error) {
	var duplicateTxIDs []flow.Identifier
	err := fork.TraverseBackward(m.headers, block.ParentID, func(ancestor *flow.Header) error {
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
// No errors are expected during normal operation.
func (m *MutableState) checkDupeTransactionsInFinalizedAncestry(
	lctx lockctx.Proof,
	includedTransactions map[flow.Identifier]struct{},
	minRefHeight, maxRefHeight, finalClusterHeight uint64,
) ([]flow.Identifier, error) {
	var dupeTxIDs []flow.Identifier

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
	// 2. For T to be eligible for inclusion in collection C, _none_ of the transactions within C are allowed to be
	//    expired w.r.t. C's reference block. Hence, for collection C to potentially contain T, it must satisfy t < c + E.
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
	err := operation.LookupClusterBlocksByReferenceHeightRange(lctx, m.db.Reader(), start, end, &clusterBlockIDs)
	if err != nil {
		return nil, fmt.Errorf("could not lookup finalized cluster blocks by reference height range [%d,%d]: %w", start, end, err)
	}

	for _, blockID := range clusterBlockIDs {
		// TODO: could add LightByBlockID and retrieve only tx IDs
		payload, err := m.payloads.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve cluster payload (block_id=%x) to de-duplicate: %w", blockID, err)
		}

		// capture any transactions in the finalized block duplicating transactions in the candidate block
		var dupeTxIDsForBlock []flow.Identifier
		for _, tx := range payload.Collection.Transactions {
			txID := tx.ID()
			_, duplicated := includedTransactions[txID]
			if duplicated {
				dupeTxIDsForBlock = append(dupeTxIDsForBlock, txID)
			}
		}

		// if no duplicates were found, continue to the next block
		if len(dupeTxIDsForBlock) == 0 {
			continue
		}

		// otherwise, if any duplicates were found, confirm that the block is an ancestor of the candidate block
		// We checked that the candidate block extends the finalized state as of the beginning of the Extend process.
		// That state was captured as `extendCtx.finalizedClusterBlock`.
		// Although not currently allowed, we assume that finalization may occur concurrently with extension,
		// so a new block may have been finalized in the interim. Here we verify that, in case such a block
		// was finalized, it is still an ancestor of the candidate block.
		header, err := m.headers.ByBlockID(blockID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve header by block_id=%x: %w", blockID, err)
		}
		// Since we already checked for duplicate transactions in the unfinalized ancestry above, we
		// know that if we found a duplicate here ABOVE our view of the finalized height, it must be on a different fork
		// (otherwise we would have found it before). So, we only consider blocks at or below our view of the finalized height.
		if header.Height <= finalClusterHeight {
			dupeTxIDs = append(dupeTxIDs, dupeTxIDsForBlock...)
			// TODO: We could stop at this point since we know the candidate is invalid.
			//       We likely SHOULD stop here when permissionless LNs are available, for performance reasons.
			//       For now, we continue and obtain a complete list of duplicates for debugging purposes, since we don't expect this case to occur.
			continue
		}
	}
	return dupeTxIDs, nil
}
