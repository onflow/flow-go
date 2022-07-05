// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// FollowerState implements a lighter version of a mutable protocol state.
// When extending the state, it performs hardly any checks on the block payload.
// Instead, the FollowerState relies on the consensus nodes to run the full
// payload check. Consequently, a block B should only be considered valid, if
// a child block with a valid header is known. The child block's header
// includes quorum certificate, which proves that a super-majority of consensus
// nodes consider block B as valid.
//
// The FollowerState allows non-consensus nodes to execute fork-aware queries
// against the protocol state, while minimizing the amount of payload checks
// the non-consensus nodes have to perform.
type FollowerState struct {
	*State

	index      storage.Index
	payloads   storage.Payloads
	tracer     module.Tracer
	consumer   protocol.Consumer
	blockTimer protocol.BlockTimer
	cfg        Config
}

// MutableState implements a mutable protocol state. When extending the
// state with a new block, it checks the _entire_ block payload.
type MutableState struct {
	*FollowerState
	receiptValidator module.ReceiptValidator
	sealValidator    module.SealValidator
}

// NewFollowerState initializes a light-weight version of a mutable protocol
// state. This implementation is suitable only for NON-Consensus nodes.
func NewFollowerState(
	state *State,
	index storage.Index,
	payloads storage.Payloads,
	tracer module.Tracer,
	consumer protocol.Consumer,
	blockTimer protocol.BlockTimer,
) (*FollowerState, error) {
	followerState := &FollowerState{
		State:      state,
		index:      index,
		payloads:   payloads,
		tracer:     tracer,
		consumer:   consumer,
		blockTimer: blockTimer,
		cfg:        DefaultConfig(),
	}
	return followerState, nil
}

// NewFullConsensusState initializes a new mutable protocol state backed by a
// badger database. When extending the state with a new block, it checks the
// _entire_ block payload. Consensus nodes should use the FullConsensusState,
// while other node roles can use the lighter FollowerState.
func NewFullConsensusState(
	state *State,
	index storage.Index,
	payloads storage.Payloads,
	tracer module.Tracer,
	consumer protocol.Consumer,
	blockTimer protocol.BlockTimer,
	receiptValidator module.ReceiptValidator,
	sealValidator module.SealValidator,
) (*MutableState, error) {
	followerState, err := NewFollowerState(state, index, payloads, tracer, consumer, blockTimer)
	if err != nil {
		return nil, fmt.Errorf("initialization of Mutable Follower State failed: %w", err)
	}
	return &MutableState{
		FollowerState:    followerState,
		receiptValidator: receiptValidator,
		sealValidator:    sealValidator,
	}, nil
}

// Extend extends the protocol state of a CONSENSUS FOLLOWER. While it checks
// the validity of the header; it does _not_ check the validity of the payload.
// Instead, the consensus follower relies on the consensus participants to
// validate the full payload. Therefore, a follower a QC (i.e. a child block) as
// proof that a block is valid.
func (m *FollowerState) Extend(ctx context.Context, candidate *flow.Block) error {

	span, ctx := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorHeaderExtend)
	defer span.Finish()

	// check if the block header is a valid extension of the finalized state
	err := m.headerExtend(candidate)
	if err != nil {
		return fmt.Errorf("header does not compliance the chain state: %w", err)
	}

	// find the last seal at the parent block
	last, err := m.lastSealed(candidate)
	if err != nil {
		return fmt.Errorf("seal in parent block does not compliance the chain state: %w", err)
	}

	// insert the block and index the last seal for the block
	err = m.insert(ctx, candidate, last)
	if err != nil {
		return fmt.Errorf("failed to insert the block: %w", err)
	}

	return nil
}

// Extend extends the protocol state of a CONSENSUS PARTICIPANT. It checks
// the validity of the _entire block_ (header and full payload).
func (m *MutableState) Extend(ctx context.Context, candidate *flow.Block) error {

	span, ctx := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtend)
	defer span.Finish()

	// check if the block header is a valid extension of the finalized state
	err := m.headerExtend(candidate)
	if err != nil {
		return fmt.Errorf("header does not compliance the chain state: %w", err)
	}

	// check if the guarantees in the payload is a valid extension of the finalized state
	err = m.guaranteeExtend(ctx, candidate)
	if err != nil {
		return fmt.Errorf("guarantee does not compliance the chain state: %w", err)
	}

	// check if the receipts in the payload are valid
	err = m.receiptExtend(ctx, candidate)
	if err != nil {
		return fmt.Errorf("payload receipts not compliant with chain state: %w", err)
	}

	// check if the seals in the payload is a valid extension of the finalized state
	lastSeal, err := m.sealExtend(ctx, candidate)
	if err != nil {
		return fmt.Errorf("seal in parent block does not compliance the chain state: %w", err)
	}

	// insert the block and index the last seal for the block
	err = m.insert(ctx, candidate, lastSeal)
	if err != nil {
		return fmt.Errorf("failed to insert the block: %w", err)
	}

	return nil
}

// headerExtend verifies the validity of the block header (excluding verification of the
// consensus rules). Specifically, we check that the block connects to the last finalized block.
func (m *FollowerState) headerExtend(candidate *flow.Block) error {
	// FIRST: We do some initial cheap sanity checks, like checking the payload
	// hash is consistent

	header := candidate.Header
	payload := candidate.Payload
	if payload.Hash() != header.PayloadHash {
		return state.NewInvalidExtensionError("payload integrity check failed")
	}

	// SECOND: Next, we can check whether the block is a valid descendant of the
	// parent. It should have the same chain ID and a height that is one bigger.

	parent, err := m.headers.ByBlockID(header.ParentID)
	if err != nil {
		return state.NewInvalidExtensionErrorf("could not retrieve parent: %s", err)
	}
	if header.ChainID != parent.ChainID {
		return state.NewInvalidExtensionErrorf("candidate built for invalid chain (candidate: %s, parent: %s)",
			header.ChainID, parent.ChainID)
	}
	if header.Height != parent.Height+1 {
		return state.NewInvalidExtensionErrorf("candidate built with invalid height (candidate: %d, parent: %d)",
			header.Height, parent.Height)
	}

	// check validity of block timestamp using parent's timestamp
	err = m.blockTimer.Validate(parent.Timestamp, candidate.Header.Timestamp)
	if err != nil {
		if protocol.IsInvalidBlockTimestampError(err) {
			return state.NewInvalidExtensionErrorf("candidate contains invalid timestamp: %w", err)
		}
		return fmt.Errorf("validating block's time stamp failed with unexpected error: %w", err)
	}

	// THIRD: Once we have established the block is valid within itself, and the
	// block is valid in relation to its parent, we can check whether it is
	// valid in the context of the entire state. For this, the block needs to
	// directly connect, through its ancestors, to the last finalized block.

	var finalizedHeight uint64
	err = m.db.View(operation.RetrieveFinalizedHeight(&finalizedHeight))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = m.db.View(operation.LookupBlockHeight(finalizedHeight, &finalID))
	if err != nil {
		return fmt.Errorf("could not lookup finalized block: %w", err)
	}

	ancestorID := header.ParentID
	for ancestorID != finalID {
		ancestor, err := m.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor (%x): %w", ancestorID, err)
		}
		if ancestor.Height < finalizedHeight {
			// this happens when the candidate block is on a fork that does not include all the
			// finalized blocks.
			// for instance:
			// A (Finalized) <- B (Finalized) <- C (Finalized) <- D <- E <- F
			//                  ^- G             ^- H             ^- I
			// block G is not a valid block, because it does not include C which has been finalized.
			// block H and I are a valid, because its their includes C.
			return state.NewOutdatedExtensionErrorf(
				"candidate block (height: %d) conflicts with finalized state (ancestor: %d final: %d)",
				header.Height, ancestor.Height, finalizedHeight)
		}
		ancestorID = ancestor.ParentID
	}

	return nil
}

// guaranteeExtend verifies the validity of the collection guarantees that are
// included in the block. Specifically, we check for expired collections and
// duplicated collections (also including ancestor blocks).
func (m *MutableState) guaranteeExtend(ctx context.Context, candidate *flow.Block) error {

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckGuarantees)
	defer span.Finish()

	header := candidate.Header
	payload := candidate.Payload

	// we only look as far back for duplicates as the transaction expiry limit;
	// if a guarantee was included before that, we will disqualify it on the
	// basis of the reference block anyway
	limit := header.Height - m.cfg.transactionExpiry
	if limit > header.Height { // overflow check
		limit = 0
	}

	// look up the root height so we don't look too far back
	// initially this is the genesis block height (aka 0).
	var rootHeight uint64
	err := m.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return fmt.Errorf("could not retrieve root block height: %w", err)
	}
	if limit < rootHeight {
		limit = rootHeight
	}

	// build a list of all previously used guarantees on this part of the chain
	ancestorID := header.ParentID
	lookup := make(map[flow.Identifier]struct{})
	for {
		ancestor, err := m.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor header (%x): %w", ancestorID, err)
		}
		index, err := m.index.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor index (%x): %w", ancestorID, err)
		}
		for _, collID := range index.CollectionIDs {
			lookup[collID] = struct{}{}
		}
		if ancestor.Height <= limit {
			break
		}
		ancestorID = ancestor.ParentID
	}

	// check each guarantee included in the payload for duplication and expiry
	for _, guarantee := range payload.Guarantees {

		// if the guarantee was already included before, error
		_, duplicated := lookup[guarantee.ID()]
		if duplicated {
			return state.NewInvalidExtensionErrorf("payload includes duplicate guarantee (%x)", guarantee.ID())
		}

		// get the reference block to check expiry
		ref, err := m.headers.ByBlockID(guarantee.ReferenceBlockID)
		if err != nil {
			return fmt.Errorf("could not get reference block (%x): %w", guarantee.ReferenceBlockID, err)
		}

		// if the guarantee references a block with expired height, error
		if ref.Height < limit {
			return state.NewInvalidExtensionErrorf("payload includes expired guarantee (height: %d, limit: %d)",
				ref.Height, limit)
		}
	}

	return nil
}

// sealExtend checks the compliance of the payload seals. Returns last seal that form a chain for
// candidate block.
func (m *MutableState) sealExtend(ctx context.Context, candidate *flow.Block) (*flow.Seal, error) {

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckSeals)
	defer span.Finish()

	lastSeal, err := m.sealValidator.Validate(candidate)
	if err != nil {
		return nil, state.NewInvalidExtensionErrorf("seal validation error: %w", err)
	}

	return lastSeal, nil
}

// receiptExtend checks the compliance of the receipt payload.
//   * Receipts should pertain to blocks on the fork
//   * Receipts should not appear more than once on a fork
//   * Receipts should pass the ReceiptValidator check
//   * No seal has been included for the respective block in this particular fork
// We require the receipts to be sorted by block height (within a payload).
func (m *MutableState) receiptExtend(ctx context.Context, candidate *flow.Block) error {

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckReceipts)
	defer span.Finish()

	err := m.receiptValidator.ValidatePayload(candidate)
	if err != nil {
		// TODO: this might be not an error, potentially it can be solved by requesting more data and processing this receipt again
		if errors.Is(err, storage.ErrNotFound) {
			return state.NewInvalidExtensionErrorf("some entities referenced by receipts are missing: %w", err)
		}
		if engine.IsInvalidInputError(err) {
			return state.NewInvalidExtensionErrorf("payload includes invalid receipts: %w", err)
		}
		return fmt.Errorf("unexpected payload validation error %w", err)
	}

	return nil
}

// lastSealed returns the highest sealed block from the fork with head `candidate`.
// For instance, here is the chain state: block 100 is the head, block 97 is finalized,
// and 95 is the last sealed block at the state of block 100.
// 95 (sealed) <- 96 <- 97 (finalized) <- 98 <- 99 <- 100
// Now, if block 101 is extending block 100, and its payload has a seal for 96, then it will
// be the last sealed for block 101.
func (m *FollowerState) lastSealed(candidate *flow.Block) (*flow.Seal, error) {
	header := candidate.Header
	payload := candidate.Payload

	// getting the last sealed block
	last, err := m.seals.ByBlockID(header.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", header.ParentID, err)
	}

	// if the payload of the block has seals, then the last seal is the seal for the highest
	// block
	if len(payload.Seals) > 0 {
		var highestHeader *flow.Header
		for i, seal := range payload.Seals {
			header, err := m.headers.ByBlockID(seal.BlockID)
			if err != nil {
				return nil, state.NewInvalidExtensionErrorf("could not retrieve the header %v for seal: %w", seal.BlockID, err)
			}

			if i == 0 || header.Height > highestHeader.Height {
				highestHeader = header
				last = seal
			}
		}
	}

	return last, nil
}

// insert stores the candidate block in the database.
// The `candidate` block _must be valid_ (otherwise, the state will be corrupted).
// dbUpdates contains other database operations which must be applied atomically
// with inserting the block.
func (m *FollowerState) insert(ctx context.Context, candidate *flow.Block, last *flow.Seal) error {

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendDBInsert)
	defer span.Finish()

	blockID := candidate.ID()
	latestSealID := last.ID()

	// apply any state changes from service events sealed by this block's parent
	dbUpdates, insertingBlockTriggersEpochFallback, err := m.handleEpochServiceEvents(candidate)
	if err != nil {
		return fmt.Errorf("could not process service events: %w", err)
	}

	// Both the header itself and its payload are in compliance with the protocol state.
	// We can now store the candidate block, as well as adding its final seal
	// to the seal index and initializing its children index.
	err = operation.RetryOnConflictTx(m.db, transaction.Update, func(tx *transaction.Tx) error {
		// insert the block into the database AND cache
		err := m.blocks.StoreTx(candidate)(tx)
		if err != nil {
			return fmt.Errorf("could not store candidate block: %w", err)
		}

		// index the latest sealed block in this fork
		err = transaction.WithTx(operation.IndexBlockSeal(blockID, latestSealID))(tx)
		if err != nil {
			return fmt.Errorf("could not index candidate seal: %w", err)
		}

		// index the child block for recovery
		err = transaction.WithTx(procedure.IndexNewBlock(blockID, candidate.Header.ParentID))(tx)
		if err != nil {
			return fmt.Errorf("could not index new block: %w", err)
		}

		// if applicable, flag that epoch fallback has been triggered
		if insertingBlockTriggersEpochFallback {
			err = transaction.WithTx(operation.SetEpochEmergencyFallbackTriggered(blockID))(tx)
			if err != nil {
				return fmt.Errorf("could not set epoch fallback triggered flag: %w", err)
			}
		}

		// apply any optional DB operations from service events
		for _, apply := range dbUpdates {
			err := apply(tx)
			if err != nil {
				return fmt.Errorf("could not apply operation: %w", err)
			}
		}

		// emit protocol events
		if insertingBlockTriggersEpochFallback {
			m.consumer.EpochEmergencyFallbackTriggered()
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("could not execute state extension: %w", err)
	}

	return nil
}

// Finalize marks the specified block as finalized.
// This method only finalizes one block at a time.
// Hence, the parent of `blockID` has to be the last finalized block.
func (m *FollowerState) Finalize(ctx context.Context, blockID flow.Identifier) error {

	// preliminaries: start tracer and retrieve full block
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorFinalize)
	defer span.Finish()
	block, err := m.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve full block that should be finalized: %w", err)
	}
	header := block.Header

	// keep track of metrics updates and protocol events to emit:
	// * metrics are updated after a successful database update
	// * protocol events are emitted atomically with the database update
	var metrics []func()
	var events []func()

	// Verify that the parent block is the latest finalized block.
	// this must be the case, as the `Finalize` method only finalizes one block
	// at a time and hence the parent of `blockID` must already be finalized.
	var finalized uint64
	err = m.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = m.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return fmt.Errorf("could not retrieve final header: %w", err)
	}
	if header.ParentID != finalID {
		return fmt.Errorf("can only finalize child of last finalized block")
	}

	// We also want to update the last sealed height. Retrieve the block
	// seal indexed for the block and retrieve the block that was sealed by it.
	last, err := m.seals.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not look up sealed header: %w", err)
	}
	sealed, err := m.headers.ByBlockID(last.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed header: %w", err)
	}

	// We update metrics and emit protocol events for epoch state changes when
	// the block corresponding to the state change is finalized
	epochStatus, err := m.epoch.statuses.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve epoch state: %w", err)
	}
	currentEpochSetup, err := m.epoch.setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		return fmt.Errorf("could not retrieve setup event for current epoch: %w", err)
	}
	epochFallbackTriggered, err := m.isEpochEmergencyFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not check persisted epoch emergency fallback flag: %w", err)
	}

	// if epoch fallback was not previously triggered, check whether this block triggers it
	if !epochFallbackTriggered {
		epochFallbackTriggered, err = m.epochFallbackTriggeredByFinalizedBlock(header, epochStatus, currentEpochSetup)
		if err != nil {
			return fmt.Errorf("could not check whether finalized block triggers epoch fallback: %w", err)
		}
		if epochFallbackTriggered {
			// emit the protocol event only the first time epoch fallback is triggered
			events = append(events, m.consumer.EpochEmergencyFallbackTriggered)
		}
	}

	// Determine metric updates and protocol events related to epoch phase
	// changes and epoch transitions.
	// If epoch emergency fallback is triggered, the current epoch continues until
	// the next spork - so skip these updates.
	if !epochFallbackTriggered {
		epochPhaseMetrics, epochPhaseEvents, err := m.epochPhaseMetricsAndEventsOnBlockFinalized(header, epochStatus)
		if err != nil {
			return fmt.Errorf("could not determine epoch phase metrics/events for finalized block: %w", err)
		}
		metrics = append(metrics, epochPhaseMetrics...)
		events = append(events, epochPhaseEvents...)

		epochTransitionMetrics, epochTransitionEvents, err := m.epochTransitionMetricsAndEventsOnBlockFinalized(header, currentEpochSetup)
		if err != nil {
			return fmt.Errorf("could not determine epoch transition metrics/events for finalized block: %w", err)
		}
		metrics = append(metrics, epochTransitionMetrics...)
		events = append(events, epochTransitionEvents...)
	}

	// if epoch emergency fallback is triggered, update metric
	if epochFallbackTriggered {
		metrics = append(metrics, m.metrics.EpochEmergencyFallbackTriggered)
	}

	// Persist updates in database
	// * Add this block to the height-indexed set of finalized blocks.
	// * Update the largest finalized height to this block's height.
	// * Update the largest height of sealed and finalized block.
	//   This value could actually stay the same if it has no seals in
	//   its payload, in which case the parent's seal is the same.
	// * set the epoch fallback flag, if it is triggered
	err = operation.RetryOnConflict(m.db.Update, func(tx *badger.Txn) error {
		err = operation.IndexBlockHeight(header.Height, blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not insert number mapping: %w", err)
		}
		err = operation.UpdateFinalizedHeight(header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not update finalized height: %w", err)
		}
		err = operation.UpdateSealedHeight(sealed.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not update sealed height: %w", err)
		}
		if epochFallbackTriggered {
			err = operation.SetEpochEmergencyFallbackTriggered(blockID)(tx)
			if err != nil {
				return fmt.Errorf("could not set epoch fallback flag: %w", err)
			}
		}

		// emit protocol events within the scope of the Badger transaction to
		// guarantee at-least-once delivery
		m.consumer.BlockFinalized(header)
		for _, emit := range events {
			emit()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not persist finalization operations for block (%x): %w", blockID, err)
	}

	// update sealed/finalized block metrics
	m.metrics.FinalizedHeight(header.Height)
	m.metrics.SealedHeight(sealed.Height)
	m.metrics.BlockFinalized(block)
	for _, seal := range block.Payload.Seals {
		sealedBlock, err := m.blocks.ByID(seal.BlockID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed block (%x): %w", seal.BlockID, err)
		}
		m.metrics.BlockSealed(sealedBlock)
	}

	// apply all queued metrics
	for _, updateMetric := range metrics {
		updateMetric()
	}

	return nil
}

// epochFallbackTriggeredByFinalizedBlock checks whether finalizing the input block
// would trigger epoch emergency fallback mode. In particular, we trigger epoch
// fallback mode when finalizing a block B when:
// 1. B is the first finalized block with view greater than or equal to the epoch
//    commitment deadline for the current epoch.
// 2. The next epoch has not been committed as of B.
//
// This function should only be called when epoch fallback *has not already been triggered*.
// See protocol.Params for more details on the epoch commitment deadline.
//
// No errors are expected during normal operation.
func (m *FollowerState) epochFallbackTriggeredByFinalizedBlock(block *flow.Header, epochStatus *flow.EpochStatus, currentEpochSetup *flow.EpochSetup) (bool, error) {

	// 1. determine whether block B is past the epoch commitment deadline
	safetyThreshold, err := m.Params().EpochCommitSafetyThreshold()
	if err != nil {
		return false, fmt.Errorf("could not get epoch commit safety threshold: %w", err)
	}
	blockExceedsDeadline := block.View+safetyThreshold >= currentEpochSetup.FinalView

	// 2. determine whether the next epoch is committed w.r.t. block B
	currentEpochPhase, err := epochStatus.Phase()
	if err != nil {
		return false, fmt.Errorf("could not get current epoch phase: %w", err)
	}
	isNextEpochCommitted := currentEpochPhase == flow.EpochPhaseCommitted

	blockTriggersEpochFallback := blockExceedsDeadline && !isNextEpochCommitted
	return blockTriggersEpochFallback, nil
}

// epochTransitionMetricsAndEventsOnBlockFinalized determines metrics to update
// and protocol events to emit, if this block is the first of a new epoch.
//
// Protocol events and updating metrics happen when we finalize the _first_
// block of the new Epoch (same convention as for Epoch-Phase-Changes).
// Approach: We retrieve the parent block's epoch information.
// When this block's view exceeds the parent epoch's final view, this block
// represents the first block of the next epoch. Therefore we update metrics
// related to the epoch transition here.
//
// This function should only be called when epoch fallback *has not already been triggered*.
// No errors are expected during normal operation.
func (m *FollowerState) epochTransitionMetricsAndEventsOnBlockFinalized(block *flow.Header, currentEpochSetup *flow.EpochSetup) (
	metrics []func(),
	events []func(),
	err error,
) {

	parentBlocksEpoch := m.AtBlockID(block.ParentID).Epochs().Current()
	parentEpochFinalView, err := parentBlocksEpoch.FinalView()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get parent epoch final view: %w", err)
	}

	if block.View > parentEpochFinalView {
		events = append(events, func() { m.consumer.EpochTransition(currentEpochSetup.Counter, block) })

		// set current epoch counter corresponding to new epoch
		metrics = append(metrics, func() { m.metrics.CurrentEpochCounter(currentEpochSetup.Counter) })
		// set epoch phase - since we are starting a new epoch we begin in the staking phase
		metrics = append(metrics, func() { m.metrics.CurrentEpochPhase(flow.EpochPhaseStaking) })
		// set current epoch view values
		metrics = append(
			metrics,
			func() { m.metrics.CurrentEpochFinalView(currentEpochSetup.FinalView) },
			func() { m.metrics.CurrentDKGPhase1FinalView(currentEpochSetup.DKGPhase1FinalView) },
			func() { m.metrics.CurrentDKGPhase2FinalView(currentEpochSetup.DKGPhase2FinalView) },
			func() { m.metrics.CurrentDKGPhase3FinalView(currentEpochSetup.DKGPhase3FinalView) },
		)
	}

	return
}

// epochPhaseMetricsAndEventsOnBlockFinalized determines metrics to update
// and protocol events to emit, if this block is the first of a new epoch phase.
//
// Protocol events and metric updates happen when we finalize the block at
// which a service event causing an epoch phase change comes into effect.
// See handleEpochServiceEvents for details.
//
// Convention:
//           A <-- ... <-- P(Seal_A) <----- B
//                         ↑                ↑
//           block con service event        first block of new Epoch phase
//           for epoch-phase transition     (e.g. EpochSetup phase)
//           (e.g. EpochSetup event)
//
// Per convention, protocol events for epoch phase changes are emitted when
// the first block of the new phase (eg. EpochSetup phase) is _finalized_.
// Meaning that the new phase has started.
//
// This function should only be called when epoch fallback *has not already been triggered*.
// No errors are expected during normal operation.
func (m *FollowerState) epochPhaseMetricsAndEventsOnBlockFinalized(block *flow.Header, epochStatus *flow.EpochStatus) (
	metrics []func(),
	events []func(),
	err error,
) {

	parent, err := m.blocks.ByID(block.ParentID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get parent (id=%x): %w", block.ParentID, err)
	}

	// track service event driven metrics and protocol events that should be emitted
	for _, seal := range parent.Payload.Seals {

		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not retrieve result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}
		for _, event := range result.ServiceEvents {
			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				// update current epoch phase
				events = append(events, func() { m.metrics.CurrentEpochPhase(flow.EpochPhaseSetup) })
				// track epoch phase transition (staking->setup)
				events = append(events, func() { m.consumer.EpochSetupPhaseStarted(ev.Counter-1, block) })
			case *flow.EpochCommit:
				// update current epoch phase
				events = append(events, func() { m.metrics.CurrentEpochPhase(flow.EpochPhaseCommitted) })
				// track epoch phase transition (setup->committed)
				events = append(events, func() { m.consumer.EpochCommittedPhaseStarted(ev.Counter-1, block) })
				// track final view of committed epoch
				nextEpochSetup, err := m.epoch.setups.ByID(epochStatus.NextEpoch.SetupID)
				if err != nil {
					return nil, nil, fmt.Errorf("could not retrieve setup event for next epoch: %w", err)
				}
				events = append(events, func() { m.metrics.CommittedEpochFinalView(nextEpochSetup.FinalView) })
			default:
				return nil, nil, fmt.Errorf("invalid service event type in payload (%T)", event)
			}
		}
	}

	return
}

// epochStatus computes the EpochStatus for the given block *before* applying
// any service event state changes which come into effect with this block.
//
// Specifically, we must determine whether block is the first block of a new
// epoch in its respective fork. We do this by comparing the block's view to
// the Epoch data from its parent. If the block's view is _larger_ than the
// final View of the parent's epoch, the block starts a new Epoch.
//
// Possible outcomes:
// 1. Block is in same Epoch as parent (block.View < epoch.FinalView)
//    -> the parent's EpochStatus.CurrentEpoch also applies for the current block
// 2. Block enters the next Epoch (block.View ≥ epoch.FinalView)
//    a) HAPPY PATH: Epoch fallback is not triggered, we enter the next epoch:
//       -> the parent's EpochStatus.NextEpoch is the current block's EpochStatus.CurrentEpoch
//    b) FALLBACK PATH: Epoch fallback is triggered, we continue the current epoch:
//       -> the parent's EpochStatus.CurrentEpoch also applies for the current block
//
// As the parent was a valid extension of the chain, by induction, the parent
// satisfies all consistency requirements of the protocol.
//
// Return values:
//  1. EpochStatus for `block`
//  2. boolean flag indicating whether we are in EECC
// No error returns are expected under normal operations
func (m *FollowerState) epochStatus(block *flow.Header) (*flow.EpochStatus, bool, error) {
	parentStatus, err := m.epoch.statuses.ByBlockID(block.ParentID)
	if err != nil {
		return nil, false, fmt.Errorf("could not retrieve epoch state for parent: %w", err)
	}
	parentSetup, err := m.epoch.setups.ByID(parentStatus.CurrentEpoch.SetupID)
	if err != nil {
		return nil, false, fmt.Errorf("could not retrieve EpochSetup event for parent: %w", err)
	}
	epochFallbackTriggered, err := m.isEpochEmergencyFallbackTriggered()
	if err != nil {
		return nil, false, fmt.Errorf("could not retrieve epoch fallback status: %w", err)
	}

	// Case 1 or 2b (still in parent block's epoch or epoch fallback triggered):
	if block.View <= parentSetup.FinalView || epochFallbackTriggered {
		// IMPORTANT: copy the status to avoid modifying the parent status in the cache
		return parentStatus.Copy(), epochFallbackTriggered, nil
	}

	// Case 2a (first block of new epoch):
	// sanity check: parent's epoch Preparation should be completed and have EpochSetup and EpochCommit events
	if parentStatus.NextEpoch.SetupID == flow.ZeroID {
		return nil, false, fmt.Errorf("missing setup event for starting next epoch")
	}
	if parentStatus.NextEpoch.CommitID == flow.ZeroID {
		return nil, false, fmt.Errorf("missing commit event for starting next epoch")
	}
	epochStatus, err := flow.NewEpochStatus(
		parentStatus.CurrentEpoch.SetupID, parentStatus.CurrentEpoch.CommitID,
		parentStatus.NextEpoch.SetupID, parentStatus.NextEpoch.CommitID,
		flow.ZeroID, flow.ZeroID,
	)
	return epochStatus, epochFallbackTriggered, err

}

// handleEpochServiceEvents handles applying state changes which occur as a result
// of service events being included in a block payload:
// * inserting incorporated service events
// * updating EpochStatus for the candidate block
//
// Consider a chain where a service event is emitted during execution of block A.
// Block B contains a receipt for A. Block C contains a seal for block A. Block
// D contains a QC for C.
//
// A <- B(RA) <- C(SA) <- D
//
// Service events are included within execution results, which are stored
// opaquely as part of the block payload in block B. We only validate and insert
// the typed service event to storage once we have received a valid QC for the
// block containing the seal for A. This occurs once we mark block D as valid
// with MarkValid. Because of this, any change to the protocol state introduced
// by a service event emitted in A would only become visible when querying D or
// later (D's children).
//
// This method will only apply service-event-induced state changes when the
// input block has the form of block D (ie. has a parent, which contains a seal
// for a block in which a service event was emitted).
//
// Return values:
//  * dbUpdates - If the service events are valid, or there are no service events,
//    this method returns a slice of Badger operations to apply while storing the block.
//    This includes an operation to index the epoch status for every block, and
//    operations to insert service events for blocks that include them.
//  * insertingCandidateTriggersEpochFallback - if inserting this block causes epoch
//    fallback mode to be triggered, this is true. This causes flag to be set in the
//    database and a protocol event to be emitted. This can only be true for at most
//    one block for a given database instance.
//
// No errors are expected during normal operation.
func (m *FollowerState) handleEpochServiceEvents(candidate *flow.Block) (
	dbUpdates []func(*transaction.Tx) error,
	insertingCandidateTriggersEpochFallback bool,
	err error,
) {
	epochStatus, isEpochFallbackTriggered, err := m.epochStatus(candidate.Header)
	if err != nil {
		return nil, false, fmt.Errorf("could not determine epoch status for candidate block: %w", err)
	}
	activeSetup, err := m.epoch.setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		return nil, false, fmt.Errorf("could not retrieve current epoch setup event: %w", err)
	}

	// always persist the candidate's epoch status
	// note: We are scheduling the operation to store the Epoch status using the _pointer_ variable `epochStatus`.
	// The struct `epochStatus` points to will still be modified below.
	blockID := candidate.ID()
	dbUpdates = append(dbUpdates, m.epoch.statuses.StoreTx(blockID, epochStatus))

	// never process service events after epoch fallback is triggered
	if isEpochFallbackTriggered {
		return
	}

	// We apply service events from blocks which are sealed by this block's PARENT.
	// The parent's payload might contain epoch preparation service events for the next
	// epoch. In this case, we need to update the tentative protocol state.
	// We need to validate whether all information is available in the protocol
	// state to go to the next epoch when needed. In cases where there is a bug
	// in the smart contract, it could be that this happens too late and the
	// chain finalization should halt.
	parent, err := m.blocks.ByID(candidate.Header.ParentID)
	if err != nil {
		return nil, false, fmt.Errorf("could not get parent (id=%x): %w", candidate.Header.ParentID, err)
	}
	for _, seal := range parent.Payload.Seals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return nil, false, fmt.Errorf("could not get result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}

		for _, event := range result.ServiceEvents {

			switch ev := event.Event.(type) {
			case *flow.EpochSetup:

				// validate the service event
				err := isValidExtendingEpochSetup(ev, activeSetup, epochStatus)
				if protocol.IsInvalidServiceEventError(err) {
					// we have observed an invalid service event, which triggers epoch fallback mode
					return dbUpdates, true, nil
				}
				if err != nil {
					return nil, false, fmt.Errorf("unexpected error validating EpochSetup service event: %w", err)
				}

				// prevents multiple setup events for same Epoch (including multiple setup events in payload of same block)
				epochStatus.NextEpoch.SetupID = ev.ID()

				// we'll insert the setup event when we insert the block
				dbUpdates = append(dbUpdates, m.epoch.setups.StoreTx(ev))

			case *flow.EpochCommit:

				extendingSetup, err := m.epoch.setups.ByID(epochStatus.NextEpoch.SetupID)
				if err != nil {
					return nil, false, state.NewInvalidExtensionErrorf("could not retrieve next epoch setup: %s", err)
				}
				// validate the service event
				err = isValidExtendingEpochCommit(ev, extendingSetup, activeSetup, epochStatus)
				if protocol.IsInvalidServiceEventError(err) {
					// we have observed an invalid service event, which triggers epoch fallback mode
					return dbUpdates, true, nil
				}
				if err != nil {
					return nil, false, fmt.Errorf("unexpected error validating EpochCommit service event: %w", err)
				}

				// prevents multiple setup events for same Epoch (including multiple setup events in payload of same block)
				epochStatus.NextEpoch.CommitID = ev.ID()

				// we'll insert the commit event when we insert the block
				dbUpdates = append(dbUpdates, m.epoch.commits.StoreTx(ev))

			default:
				return nil, false, fmt.Errorf("invalid service event type: %s", event.Type)
			}
		}
	}
	return
}

// MarkValid marks the block as valid in protocol state, and triggers
// `BlockProcessable` event to notify that its parent block is processable.
//
// Why is the parent block processable, not the block itself?
// because a block having a child block means it has been verified
// by the majority of consensus participants.
// Hence, if a block has passed the header validity check, its parent block
// must have passed both the header validity check and the body validity check.
// So that consensus followers can skip the block body validity checks and wait
// for its child to arrive, and if the child passes the header validity check, it means
// the consensus participants have done a complete check on its parent block,
// so consensus followers can trust consensus nodes did the right job, and start
// processing the parent block.
//
// NOTE: since a parent can have multiple children, `BlockProcessable` event
// could be triggered multiple times for the same block.
// NOTE: BlockProcessable should not be blocking, otherwise, it will block the follower
//
func (m *FollowerState) MarkValid(blockID flow.Identifier) error {
	header, err := m.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve block header for %x: %w", blockID, err)
	}
	parentID := header.ParentID
	var isParentValid bool
	err = m.db.View(operation.RetrieveBlockValidity(parentID, &isParentValid))
	if err != nil {
		return fmt.Errorf("could not retrieve validity of parent block (%x): %w", parentID, err)
	}
	if !isParentValid {
		return fmt.Errorf("can only mark block as valid whose parent is valid")
	}

	parent, err := m.headers.ByBlockID(parentID)
	if err != nil {
		return fmt.Errorf("could not retrieve block header for %x: %w", parentID, err)
	}
	// root blocks and blocks below the root block are considered as "processed",
	// so we don't want to trigger `BlockProcessable` event for them.
	var rootHeight uint64
	err = m.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return fmt.Errorf("could not retrieve root block's height: %w", err)
	}

	err = operation.RetryOnConflict(m.db.Update, func(tx *badger.Txn) error {
		// insert block validity for this block
		err = operation.SkipDuplicates(operation.InsertBlockValidity(blockID, true))(tx)
		if err != nil {
			return fmt.Errorf("could not insert validity for block (id=%x, height=%d): %w", blockID, header.Height, err)
		}

		// trigger BlockProcessable for parent blocks above root height
		if parent.Height > rootHeight {
			// emit protocol event within the scope of the Badger transaction to
			// guarantee at-least-once delivery
			m.consumer.BlockProcessable(parent)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not mark block as valid (%x): %w", blockID, err)
	}

	return nil
}
