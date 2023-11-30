package badger

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/signature"
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
// payload check and uses quorum certificates to prove validity of block payloads.
// Consequently, a block B should only be considered valid, if
// there is a certifying QC for that block QC.View == Block.View && QC.BlockID == Block.ID().
//
// The FollowerState allows non-consensus nodes to execute fork-aware queries
// against the protocol state, while minimizing the amount of payload checks
// the non-consensus nodes have to perform.
type FollowerState struct {
	*State

	index      storage.Index
	payloads   storage.Payloads
	tracer     module.Tracer
	logger     zerolog.Logger
	consumer   protocol.Consumer
	blockTimer protocol.BlockTimer
}

var _ protocol.FollowerState = (*FollowerState)(nil)

// ParticipantState implements a mutable state for consensus participant. It can extend the
// state with a new block, by checking the _entire_ block payload.
type ParticipantState struct {
	*FollowerState
	receiptValidator module.ReceiptValidator
	sealValidator    module.SealValidator
}

var _ protocol.ParticipantState = (*ParticipantState)(nil)

// NewFollowerState initializes a light-weight version of a mutable protocol
// state. This implementation is suitable only for NON-Consensus nodes.
func NewFollowerState(
	logger zerolog.Logger,
	tracer module.Tracer,
	consumer protocol.Consumer,
	state *State,
	index storage.Index,
	payloads storage.Payloads,
	blockTimer protocol.BlockTimer,
) (*FollowerState, error) {
	followerState := &FollowerState{
		State:      state,
		index:      index,
		payloads:   payloads,
		tracer:     tracer,
		logger:     logger,
		consumer:   consumer,
		blockTimer: blockTimer,
	}
	return followerState, nil
}

// NewFullConsensusState initializes a new mutable protocol state backed by a
// badger database. When extending the state with a new block, it checks the
// _entire_ block payload. Consensus nodes should use the FullConsensusState,
// while other node roles can use the lighter FollowerState.
func NewFullConsensusState(
	logger zerolog.Logger,
	tracer module.Tracer,
	consumer protocol.Consumer,
	state *State,
	index storage.Index,
	payloads storage.Payloads,
	blockTimer protocol.BlockTimer,
	receiptValidator module.ReceiptValidator,
	sealValidator module.SealValidator,
) (*ParticipantState, error) {
	followerState, err := NewFollowerState(
		logger,
		tracer,
		consumer,
		state,
		index,
		payloads,
		blockTimer,
	)
	if err != nil {
		return nil, fmt.Errorf("initialization of Mutable Follower State failed: %w", err)
	}
	return &ParticipantState{
		FollowerState:    followerState,
		receiptValidator: receiptValidator,
		sealValidator:    sealValidator,
	}, nil
}

// ExtendCertified extends the protocol state of a CONSENSUS FOLLOWER. While it checks
// the validity of the header; it does _not_ check the validity of the payload.
// Instead, the consensus follower relies on the consensus participants to
// validate the full payload. Payload validity can be proved by a valid quorum certificate.
// Certifying QC must match candidate block:
//
//	candidate.View == certifyingQC.View && candidate.ID() == certifyingQC.BlockID
//
// Caution:
//   - This function expects that `certifyingQC` has been validated.
//   - The parent block must already be stored.
//
// No errors are expected during normal operations.
func (m *FollowerState) ExtendCertified(ctx context.Context, candidate *flow.Block, certifyingQC *flow.QuorumCertificate) error {
	span, ctx := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorHeaderExtend)
	defer span.End()

	// check if candidate block has been already processed
	blockID := candidate.ID()
	isDuplicate, err := m.checkBlockAlreadyProcessed(blockID)
	if err != nil || isDuplicate {
		return err
	}

	// sanity check if certifyingQC actually certifies candidate block
	if certifyingQC.View != candidate.Header.View {
		return fmt.Errorf("qc doesn't certify candidate block, expect %d view, got %d", candidate.Header.View, certifyingQC.View)
	}
	if certifyingQC.BlockID != blockID {
		return fmt.Errorf("qc doesn't certify candidate block, expect %x blockID, got %x", blockID, certifyingQC.BlockID)
	}

	// check if the block header is a valid extension of parent block
	err = m.headerExtend(candidate)
	if err != nil {
		// since we have a QC for this block, it cannot be an invalid extension
		return fmt.Errorf("unexpected invalid block (id=%x) with certifying qc (id=%x): %s",
			candidate.ID(), certifyingQC.ID(), err.Error())
	}

	// find the last seal at the parent block
	last, err := m.lastSealed(candidate)
	if err != nil {
		return fmt.Errorf("payload seal(s) not compliant with chain state: %w", err)
	}

	// insert the block, certifying QC and index the last seal for the block
	err = m.insert(ctx, candidate, certifyingQC, last)
	if err != nil {
		return fmt.Errorf("failed to insert the block: %w", err)
	}

	return nil
}

// Extend extends the protocol state of a CONSENSUS PARTICIPANT. It checks
// the validity of the _entire block_ (header and full payload).
// Expected errors during normal operations:
//   - state.OutdatedExtensionError if the candidate block is outdated (e.g. orphaned)
//   - state.InvalidExtensionError if the candidate block is invalid
func (m *ParticipantState) Extend(ctx context.Context, candidate *flow.Block) error {
	span, ctx := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtend)
	defer span.End()

	// check if candidate block has been already processed
	isDuplicate, err := m.checkBlockAlreadyProcessed(candidate.ID())
	if err != nil || isDuplicate {
		return err
	}

	// check if the block header is a valid extension of parent block
	err = m.headerExtend(candidate)
	if err != nil {
		return fmt.Errorf("header not compliant with chain state: %w", err)
	}

	// check if the block header is a valid extension of the finalized state
	err = m.checkOutdatedExtension(candidate.Header)
	if err != nil {
		if state.IsOutdatedExtensionError(err) {
			return fmt.Errorf("candidate block is an outdated extension: %w", err)
		}
		return fmt.Errorf("could not check if block is an outdated extension: %w", err)
	}

	// check if the guarantees in the payload is a valid extension of the finalized state
	err = m.guaranteeExtend(ctx, candidate)
	if err != nil {
		return fmt.Errorf("payload guarantee(s) not compliant with chain state: %w", err)
	}

	// check if the receipts in the payload are valid
	err = m.receiptExtend(ctx, candidate)
	if err != nil {
		return fmt.Errorf("payload receipt(s) not compliant with chain state: %w", err)
	}

	// check if the seals in the payload is a valid extension of the finalized state
	lastSeal, err := m.sealExtend(ctx, candidate)
	if err != nil {
		return fmt.Errorf("payload seal(s) not compliant with chain state: %w", err)
	}

	// insert the block and index the last seal for the block
	err = m.insert(ctx, candidate, nil, lastSeal)
	if err != nil {
		return fmt.Errorf("failed to insert the block: %w", err)
	}

	return nil
}

// headerExtend verifies the validity of the block header (excluding verification of the
// consensus rules). Specifically, we check that the block connects to the last finalized block.
// Expected errors during normal operations:
//   - state.InvalidExtensionError if the candidate block is invalid
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
	if header.ParentView != parent.View {
		return state.NewInvalidExtensionErrorf("candidate build with inconsistent parent view (candidate: %d, parent %d)",
			header.ParentView, parent.View)
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

	return nil
}

// checkBlockAlreadyProcessed checks if block has been added to the protocol state.
// Returns:
// * (true, nil) - block has been already processed.
// * (false, nil) - block has not been processed.
// * (false, error) - unknown error when trying to query protocol state.
// No errors are expected during normal operation.
func (m *FollowerState) checkBlockAlreadyProcessed(blockID flow.Identifier) (bool, error) {
	_, err := m.headers.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("could not check if candidate block (%x) has been already processed: %w", blockID, err)
	}
	return true, nil
}

// checkOutdatedExtension checks whether given block is
// valid in the context of the entire state. For this, the block needs to
// directly connect, through its ancestors, to the last finalized block.
// Expected errors during normal operations:
//   - state.OutdatedExtensionError if the candidate block is outdated (e.g. orphaned)
func (m *ParticipantState) checkOutdatedExtension(header *flow.Header) error {
	var finalizedHeight uint64
	err := m.db.View(operation.RetrieveFinalizedHeight(&finalizedHeight))
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
			// block G is not a valid block, because it does not have C (which has been finalized) as an ancestor
			// block H and I are valid, because they do have C as an ancestor
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
func (m *ParticipantState) guaranteeExtend(ctx context.Context, candidate *flow.Block) error {

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckGuarantees)
	defer span.End()

	header := candidate.Header
	payload := candidate.Payload

	// we only look as far back for duplicates as the transaction expiry limit;
	// if a guarantee was included before that, we will disqualify it on the
	// basis of the reference block anyway
	limit := header.Height - flow.DefaultTransactionExpiry
	if limit > header.Height { // overflow check
		limit = 0
	}
	if limit < m.sporkRootBlockHeight {
		limit = m.sporkRootBlockHeight
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
			if errors.Is(err, storage.ErrNotFound) {
				return state.NewInvalidExtensionErrorf("could not get reference block %x: %w", guarantee.ReferenceBlockID, err)
			}
			return fmt.Errorf("could not get reference block (%x): %w", guarantee.ReferenceBlockID, err)
		}

		// if the guarantee references a block with expired height, error
		if ref.Height < limit {
			return state.NewInvalidExtensionErrorf("payload includes expired guarantee (height: %d, limit: %d)",
				ref.Height, limit)
		}

		// check the guarantors are correct
		_, err = protocol.FindGuarantors(m, guarantee)
		if err != nil {
			if signature.IsInvalidSignerIndicesError(err) ||
				errors.Is(err, protocol.ErrNextEpochNotCommitted) ||
				errors.Is(err, protocol.ErrClusterNotFound) {
				return state.NewInvalidExtensionErrorf("guarantee %v contains invalid guarantors: %w", guarantee.ID(), err)
			}
			return fmt.Errorf("could not find guarantor for guarantee %v: %w", guarantee.ID(), err)
		}
	}

	return nil
}

// sealExtend checks the compliance of the payload seals. Returns last seal that form a chain for
// candidate block.
func (m *ParticipantState) sealExtend(ctx context.Context, candidate *flow.Block) (*flow.Seal, error) {

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckSeals)
	defer span.End()

	lastSeal, err := m.sealValidator.Validate(candidate)
	if err != nil {
		return nil, state.NewInvalidExtensionErrorf("seal validation error: %w", err)
	}

	return lastSeal, nil
}

// receiptExtend checks the compliance of the receipt payload.
//   - Receipts should pertain to blocks on the fork
//   - Receipts should not appear more than once on a fork
//   - Receipts should pass the ReceiptValidator check
//   - No seal has been included for the respective block in this particular fork
//
// We require the receipts to be sorted by block height (within a payload).
func (m *ParticipantState) receiptExtend(ctx context.Context, candidate *flow.Block) error {

	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckReceipts)
	defer span.End()

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
// No errors are expected during normal operation.
func (m *FollowerState) lastSealed(candidate *flow.Block) (*flow.Seal, error) {
	header := candidate.Header
	payload := candidate.Payload

	// getting the last sealed block
	last, err := m.seals.HighestInFork(header.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", header.ParentID, err)
	}

	// if the payload of the block has no seals, then the last seal is the seal for the highest block
	if len(payload.Seals) == 0 {
		return last, nil
	}

	ordered, err := protocol.OrderedSeals(payload.Seals, m.headers)
	if err != nil {
		// all errors are unexpected - differentiation is for clearer error messages
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("ordering seals: candidate payload contains seals for unknown block: %s", err.Error())
		}
		if errors.Is(err, protocol.ErrDiscontinuousSeals) || errors.Is(err, protocol.ErrMultipleSealsForSameHeight) {
			return nil, fmt.Errorf("ordering seals: candidate payload contains invalid seal set: %s", err.Error())
		}
		return nil, fmt.Errorf("unexpected error ordering seals: %w", err)
	}
	return ordered[len(ordered)-1], nil
}

// insert stores the candidate block in the database.
// The `candidate` block _must be valid_ (otherwise, the state will be corrupted).
// dbUpdates contains other database operations which must be applied atomically
// with inserting the block.
// Caller is responsible for ensuring block validity.
// If insert is called from Extend(by consensus participant) then certifyingQC will be nil but the block payload will be validated.
// If insert is called from ExtendCertified(by consensus follower) then certifyingQC must be not nil which proves payload validity.
// No errors are expected during normal operations.
func (m *FollowerState) insert(ctx context.Context, candidate *flow.Block, certifyingQC *flow.QuorumCertificate, last *flow.Seal) error {
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendDBInsert)
	defer span.End()

	blockID := candidate.ID()
	parentID := candidate.Header.ParentID
	latestSealID := last.ID()

	parent, err := m.headers.ByBlockID(parentID)
	if err != nil {
		return fmt.Errorf("could not retrieve block header for %x: %w", parentID, err)
	}

	stateMutator, err := m.protocolState.Mutator(candidate.Header.View, parentID)
	if err != nil {
		return fmt.Errorf("could not create protocol state mutator for view %d: %w", candidate.Header.View, err)
	}

	// apply any state changes from service events sealed by this block
	err = stateMutator.ApplyServiceEventsFromValidatedSeals(candidate.Payload.Seals)
	if err != nil {
		return fmt.Errorf("could not process service events: %w", err)
	}

	hasChanges, updatedState, updatedStateID, dbUpdates := stateMutator.Build()

	if updatedStateID != candidate.Payload.ProtocolStateID {
		return state.NewInvalidExtensionErrorf("invalid protocol state transition detected, "+
			"payload contains (%x) but after applying changes got %x", candidate.Payload.ProtocolStateID, updatedStateID)
	}
	if hasChanges {
		dbUpdates = append(dbUpdates, operation.SkipDuplicatesTx(m.protocolStateSnapshotsDB.StoreTx(updatedStateID, updatedState)))
	}
	dbUpdates = append(dbUpdates, m.protocolStateSnapshotsDB.Index(blockID, updatedStateID))

	// events is a queue of node-internal events (aka notifications) that are emitted after the database write succeeded
	var events []func()

	if certifyingQC != nil {
		dbUpdates = append(dbUpdates, m.qcs.StoreTx(certifyingQC))

		// queue an BlockProcessable event for candidate block, since it is certified
		events = append(events, func() {
			m.consumer.BlockProcessable(candidate.Header, certifyingQC)
		})
	}

	// Both the header itself and its payload are in compliance with the protocol state.
	// We can now store the candidate block, as well as adding its final seal
	// to the seal index and initializing its children index.
	qc := candidate.Header.QuorumCertificate()
	err = operation.RetryOnConflictTx(m.db, transaction.Update, func(tx *transaction.Tx) error {
		// insert the block into the database AND cache
		err := m.blocks.StoreTx(candidate)(tx)
		if err != nil {
			return fmt.Errorf("could not store candidate block: %w", err)
		}

		err = m.qcs.StoreTx(qc)(tx)
		if err != nil {
			if !errors.Is(err, storage.ErrAlreadyExists) {
				return fmt.Errorf("could not store incorporated qc: %w", err)
			}
		} else {
			// trigger BlockProcessable for parent blocks above root height
			if parent.Height > m.finalizedRootHeight {
				events = append(events, func() {
					m.consumer.BlockProcessable(parent, qc)
				})
			}
		}

		// index the latest sealed block in this fork
		err = transaction.WithTx(operation.IndexLatestSealAtBlock(blockID, latestSealID))(tx)
		if err != nil {
			return fmt.Errorf("could not index candidate seal: %w", err)
		}

		// index the child block for recovery
		err = transaction.WithTx(procedure.IndexNewBlock(blockID, candidate.Header.ParentID))(tx)
		if err != nil {
			return fmt.Errorf("could not index new block: %w", err)
		}

		// apply any optional DB operations from service events
		for _, apply := range dbUpdates {
			err := apply(tx)
			if err != nil {
				return fmt.Errorf("could not apply operation: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not execute state extension: %w", err)
	}

	// execute scheduled events
	for _, event := range events {
		event()
	}

	return nil
}

// Finalize marks the specified block as finalized.
// This method only finalizes one block at a time.
// Hence, the parent of `blockID` has to be the last finalized block.
// No errors are expected during normal operations.
func (m *FollowerState) Finalize(ctx context.Context, blockID flow.Identifier) error {

	// preliminaries: start tracer and retrieve full block
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorFinalize)
	defer span.End()
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
	lastSeal, err := m.seals.HighestInFork(blockID)
	if err != nil {
		return fmt.Errorf("could not look up sealed header: %w", err)
	}
	sealed, err := m.headers.ByBlockID(lastSeal.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed header: %w", err)
	}

	// We update metrics and emit protocol events for epoch state changes when
	// the block corresponding to the state change is finalized
	psSnapshot, err := m.protocolState.AtBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve protocol state snapshot: %w", err)
	}
	// todo: need InvalidEpochTransitionAttempted and phase
	currentEpochSetup := psSnapshot.EpochSetup()
	epochFallbackTriggered, err := m.isEpochEmergencyFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not check persisted epoch emergency fallback flag: %w", err)
	}

	// if epoch fallback was not previously triggered, check whether this block triggers it
	if !epochFallbackTriggered {
		epochFallbackTriggered, err = m.epochFallbackTriggeredByFinalizedBlock(header, psSnapshot)
		if err != nil {
			return fmt.Errorf("could not check whether finalized block triggers epoch fallback: %w", err)
		}
		if epochFallbackTriggered {
			// emit the protocol event only the first time epoch fallback is triggered
			events = append(events, m.consumer.EpochEmergencyFallbackTriggered)
			metrics = append(metrics, m.metrics.EpochEmergencyFallbackTriggered)
		}
	}

	isFirstBlockOfEpoch, err := m.isFirstBlockOfEpoch(header, currentEpochSetup)
	if err != nil {
		return fmt.Errorf("could not check if block is first of epoch: %w", err)
	}

	// Determine metric updates and protocol events related to epoch phase
	// changes and epoch transitions.
	// If epoch emergency fallback is triggered, the current epoch continues until
	// the next spork - so skip these updates.
	if !epochFallbackTriggered {
		epochPhaseMetrics, epochPhaseEvents, err := m.epochPhaseMetricsAndEventsOnBlockFinalized(block, epochStatus)
		if err != nil {
			return fmt.Errorf("could not determine epoch phase metrics/events for finalized block: %w", err)
		}
		metrics = append(metrics, epochPhaseMetrics...)
		events = append(events, epochPhaseEvents...)

		if isFirstBlockOfEpoch {
			epochTransitionMetrics, epochTransitionEvents := m.epochTransitionMetricsAndEventsOnBlockFinalized(header, psSnapshot.EpochSetup())
			if err != nil {
				return fmt.Errorf("could not determine epoch transition metrics/events for finalized block: %w", err)
			}
			metrics = append(metrics, epochTransitionMetrics...)
			events = append(events, epochTransitionEvents...)
		}
	}

	// Extract and validate version beacon events from the block seals.
	versionBeacons, err := m.versionBeaconOnBlockFinalized(block)
	if err != nil {
		return fmt.Errorf("cannot process version beacon: %w", err)
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
		if isFirstBlockOfEpoch && !epochFallbackTriggered {
			err = operation.InsertEpochFirstHeight(currentEpochSetup.Counter, header.Height)(tx)
			if err != nil {
				return fmt.Errorf("could not insert epoch first block height: %w", err)
			}
		}

		// When a block is finalized, we commit the result for each seal it contains. The sealing logic
		// guarantees that only a single, continuous execution fork is sealed. Here, we index for
		// each block ID the ID of its _finalized_ seal.
		for _, seal := range block.Payload.Seals {
			err = operation.IndexFinalizedSealByBlockID(seal.BlockID, seal.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not index the seal by the sealed block ID: %w", err)
			}
		}

		if len(versionBeacons) > 0 {
			// only index the last version beacon as that is the relevant one.
			// TODO: The other version beacons can be used for validation.
			err := operation.IndexVersionBeaconByHeight(versionBeacons[len(versionBeacons)-1])(tx)
			if err != nil {
				return fmt.Errorf("could not index version beacon or height (%d): %w", header.Height, err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not persist finalization operations for block (%x): %w", blockID, err)
	}

	// update the cache
	m.State.cachedFinal.Store(&cachedHeader{blockID, header})
	if len(block.Payload.Seals) > 0 {
		m.State.cachedSealed.Store(&cachedHeader{lastSeal.BlockID, sealed})
	}

	// Emit protocol events after database transaction succeeds. Event delivery is guaranteed,
	// _except_ in case of a crash. Hence, when recovering from a crash, consumers need to deduce
	// from the state whether they have missed events and re-execute the respective actions.
	m.consumer.BlockFinalized(header)
	for _, emit := range events {
		emit()
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
// fallback mode while finalizing block B in either of the following cases:
//  1. B is the head of a fork in which epoch fallback was tentatively triggered,
//     due to incorporating an invalid service event.
//  2. (a) B is the first finalized block with view greater than or equal to the epoch
//     commitment deadline for the current epoch AND
//     (b) the next epoch has not been committed as of B.
//
// This function should only be called when epoch fallback *has not already been triggered*.
// See protocol.Params for more details on the epoch commitment deadline.
//
// No errors are expected during normal operation.
func (m *FollowerState) epochFallbackTriggeredByFinalizedBlock(block *flow.Header, stateAtBlock protocol.DynamicProtocolState) (bool, error) {
	// todo: replace epochStatus with protocolStateEntry
	// 1. Epoch fallback is tentatively triggered on this fork
	if stateAtBlock.InvalidEpochTransitionAttempted() {
		return true, nil
	}

	// 2.(a) determine whether block B is past the epoch commitment deadline
	safetyThreshold := m.Params().EpochCommitSafetyThreshold()
	blockExceedsDeadline := block.View+safetyThreshold >= stateAtBlock.EpochSetup().FinalView

	// 2.(b) determine whether the next epoch is committed w.r.t. block B
	isNextEpochCommitted := stateAtBlock.EpochPhase() == flow.EpochPhaseCommitted

	blockTriggersEpochFallback := blockExceedsDeadline && !isNextEpochCommitted
	return blockTriggersEpochFallback, nil
}

// isFirstBlockOfEpoch returns true if the given block is the first block of a new epoch.
// We accept the EpochSetup event for the current epoch (w.r.t. input block B) which contains
// the FirstView for the epoch (denoted W). By construction, B.View >= W.
// Definition: B is the first block of the epoch if and only if B.parent.View < W
//
// NOTE: There can be multiple (un-finalized) blocks that qualify as the first block of epoch N.
// No errors are expected during normal operation.
func (m *FollowerState) isFirstBlockOfEpoch(block *flow.Header, currentEpochSetup *flow.EpochSetup) (bool, error) {
	currentEpochFirstView := currentEpochSetup.FirstView
	// sanity check: B.View >= W
	if block.View < currentEpochFirstView {
		return false, irrecoverable.NewExceptionf("data inconsistency: block (id=%x, view=%d) is below its epoch first view %d", block.ID(), block.View, currentEpochFirstView)
	}

	parent, err := m.headers.ByBlockID(block.ParentID)
	if err != nil {
		return false, irrecoverable.NewExceptionf("could not retrieve parent (id=%s): %w", block.ParentID, err)
	}

	return parent.View < currentEpochFirstView, nil
}

// epochTransitionMetricsAndEventsOnBlockFinalized determines metrics to update
// and protocol events to emit for blocks which are the first block of a new epoch.
// Protocol events and updating metrics happen once when we finalize the _first_
// block of the new Epoch (same convention as for Epoch-Phase-Changes).
//
// NOTE: This function must only be called when input `block` is the first block
// of the epoch denoted by `currentEpochSetup`.
func (m *FollowerState) epochTransitionMetricsAndEventsOnBlockFinalized(block *flow.Header, currentEpochSetup *flow.EpochSetup) (
	metrics []func(),
	events []func(),
) {

	events = append(events, func() { m.consumer.EpochTransition(currentEpochSetup.Counter, block) })
	// set current epoch counter corresponding to new epoch
	metrics = append(metrics, func() { m.metrics.CurrentEpochCounter(currentEpochSetup.Counter) })
	// denote the most recent epoch transition height
	metrics = append(metrics, func() { m.metrics.EpochTransitionHeight(block.Height) })
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

	return
}

// epochPhaseMetricsAndEventsOnBlockFinalized determines metrics to update and protocol
// events to emit. Service Events embedded into an execution result take effect, when the
// execution result's _seal is finalized_ (i.e. when the block holding a seal for the
// result is finalized). See also handleEpochServiceEvents for further details. Example:
//
// Convention:
//
//	A <-- ... <-- C(Seal_A)
//
// Suppose an EpochSetup service event is emitted during execution of block A. C seals A, therefore
// we apply the metrics/events when C is finalized. The first block of the EpochSetup
// phase is block C.
//
// This function should only be called when epoch fallback *has not already been triggered*.
// No errors are expected during normal operation.
func (m *FollowerState) epochPhaseMetricsAndEventsOnBlockFinalized(block *flow.Block, stateAtBlock protocol.DynamicProtocolState) (
	metrics []func(),
	events []func(),
	err error,
) {

	// block payload may not specify seals in order, so order them by block height before processing
	orderedSeals, err := protocol.OrderedSeals(block.Payload.Seals, m.headers)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil, fmt.Errorf("ordering seals: parent payload contains seals for unknown block: %s", err.Error())
		}
		return nil, nil, fmt.Errorf("unexpected error ordering seals: %w", err)
	}

	// track service event driven metrics and protocol events that should be emitted
	for _, seal := range orderedSeals {
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
				events = append(events, func() { m.consumer.EpochSetupPhaseStarted(ev.Counter-1, block.Header) })
			case *flow.EpochCommit:
				// update current epoch phase
				events = append(events, func() { m.metrics.CurrentEpochPhase(flow.EpochPhaseCommitted) })
				// track epoch phase transition (setup->committed)
				events = append(events, func() { m.consumer.EpochCommittedPhaseStarted(ev.Counter-1, block.Header) })
				// track final view of committed epoch
				nextEpochSetup := stateAtBlock.Entry().NextEpochSetup
				if nextEpochSetup != nil {
					return nil, nil, fmt.Errorf("we are in Epoch")
				}
				nextEpochSetup, err := m.epoch.setups.ByID(epochStatus.NextEpoch.SetupID)
				if err != nil {
					return nil, nil, fmt.Errorf("could not retrieve setup event for next epoch: %w", err)
				}
				events = append(events, func() { m.metrics.CommittedEpochFinalView(nextEpochSetup.FinalView) })
			case *flow.VersionBeacon:
				// do nothing for now
			default:
				return nil, nil, fmt.Errorf("invalid service event type in payload (%T)", ev)
			}
		}
	}

	return
}

// versionBeaconOnBlockFinalized extracts and returns the VersionBeacons from the
// finalized block's seals.
// This could return multiple VersionBeacons if the parent block contains multiple Seals.
// The version beacons will be returned in the ascending height order of the seals.
// Technically only the last VersionBeacon is relevant.
func (m *FollowerState) versionBeaconOnBlockFinalized(
	finalized *flow.Block,
) ([]*flow.SealedVersionBeacon, error) {
	var versionBeacons []*flow.SealedVersionBeacon

	seals, err := protocol.OrderedSeals(finalized.Payload.Seals, m.headers)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf(
				"ordering seals: parent payload contains"+
					" seals for unknown block: %w", err)
		}
		return nil, fmt.Errorf("unexpected error ordering seals: %w", err)
	}

	for _, seal := range seals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return nil, fmt.Errorf(
				"could not retrieve result (id=%x) for seal (id=%x): %w",
				seal.ResultID,
				seal.ID(),
				err)
		}
		for _, event := range result.ServiceEvents {

			ev, ok := event.Event.(*flow.VersionBeacon)

			if !ok {
				// skip other service event types.
				// validation if this is a known service event type is done elsewhere.
				continue
			}

			err := ev.Validate()
			if err != nil {
				m.logger.Warn().
					Err(err).
					Str("block_id", finalized.ID().String()).
					Interface("event", ev).
					Msg("invalid VersionBeacon service event")
				continue
			}

			// The version beacon only becomes actionable/valid/active once the block
			// containing the version beacon has been sealed. That is why we set the
			// Seal height to the current block height.
			versionBeacons = append(versionBeacons, &flow.SealedVersionBeacon{
				VersionBeacon: ev,
				SealHeight:    finalized.Header.Height,
			})
		}
	}

	return versionBeacons, nil
}
