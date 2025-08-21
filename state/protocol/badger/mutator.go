package badger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	protocol_state "github.com/onflow/flow-go/state/protocol/protocol_state/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/deferred"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/procedure"
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

	index         storage.Index
	payloads      storage.Payloads
	tracer        module.Tracer
	logger        zerolog.Logger
	consumer      protocol.Consumer
	blockTimer    protocol.BlockTimer
	protocolState protocol.MutableProtocolState
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
		protocolState: protocol_state.NewMutableProtocolState(
			logger,
			state.epochProtocolStateEntriesDB,
			state.protocolKVStoreSnapshotsDB,
			state.params,
			state.headers,
			state.results,
			state.epoch.setups,
			state.epoch.commits,
		),
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
// CAUTION:
//   - This function expects that `certifyingQC ` has been validated. (otherwise, the state will be corrupted)
//   - The parent block must already have been ingested.
//
// Per convention, the protocol state requires that the candidate's parent has already been ingested.
// Other than that, all valid extensions are accepted. Even if we have enough information to determine that
// a candidate block is already orphaned (e.g. its view is below the latest finalized view), it is important
// to accept it nevertheless to avoid spamming vulnerabilities. If a block is orphaned, consensus rules
// guarantee that there exists only a limited number of descendants which cannot increase anymore. So there
// is only a finite (generally small) amount of work to do accepting orphaned blocks and all their descendants.
// However, if we were to drop orphaned blocks, e.g. block X of the orphaned fork X <- Y <- Z, we might not
// have enough information to reject blocks Y, Z later if we receive them. We would re-request X, then
// determine it is orphaned and drop it, attempt to ingest Y re-request the unknown parent X and repeat
// potentially very often.
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
	deferredBlockPersist, err := m.headerExtend(ctx, candidate, certifyingQC)
	if err != nil {
		// since we have a QC for this block, it cannot be an invalid extension
		return fmt.Errorf("unexpected invalid block (id=%x) with certifying qc (id=%x): %s",
			candidate.ID(), certifyingQC.ID(), err.Error())
	}

	// find the last seal at the parent block
	latestSeal, err := m.lastSealed(candidate)
	if err != nil {
		return fmt.Errorf("failed to determine the lastest sealed block in fork: %w", err)
	}
	deferredBlockPersist.AddNextOperation(func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
		return operation.IndexLatestSealAtBlock(lctx, rw.Writer(), blockID, latestSeal.ID())
	})

	// the Extend method has already been fully constructed.
	// evolve protocol state and verify consistency with commitment included in
	deferredProtocolStatePersist, err := m.evolveProtocolState(ctx, candidate)
	if err != nil {
		return fmt.Errorf("evolving protocol state failed: %w", err)
	}
	deferredBlockPersist.Chain(deferredProtocolStatePersist)

	// Final step: for atomicity we acquire lock and then execute the deferred operations
	// within the protection of the lock.
	lctx := m.lockManager.NewContext()
	defer lctx.Release()
	err = lctx.AcquireLock(storage.LockInsertBlock)
	if err != nil {
		return err
	}

	// Execute the deferred database operations as one atomic transaction and emit scheduled notifications on success.
	// The `candidate` block _must be valid_ (otherwise, the state will be corrupted)!
	return m.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return deferredBlockPersist.Execute(lctx, blockID, rw)
	})
}

// Extend extends the protocol state of a CONSENSUS PARTICIPANT. It checks
// the validity of the _entire block_ (header and full payload).
//
// CAUTION: per convention, the protocol state requires that the candidate's
// parent has already been ingested. Otherwise, an exception is returned.
//
// Per convention, the protocol state requires that the candidate's parent has already been ingested.
// Other than that, all valid extensions are accepted. Even if we have enough information to determine that
// a candidate block is already orphaned (e.g. its view is below the latest finalized view), it is important
// to accept it nevertheless to avoid spamming vulnerabilities. If a block is orphaned, consensus rules
// guarantee that there exists only a limited number of descendants which cannot increase anymore. So there
// is only a finite (generally small) amount of work to do accepting orphaned blocks and all their descendants.
// However, if we were to drop orphaned blocks, e.g. block X of the orphaned fork X <- Y <- Z, we might not
// have enough information to reject blocks Y, Z later if we receive them. We would re-request X, then
// determine it is orphaned and drop it, attempt to ingest Y re-request the unknown parent X and repeat
// potentially very often.
//
// Expected errors during normal operations:
//   - state.OutdatedExtensionError if the candidate block is outdated (e.g. orphaned)
//   - state.InvalidExtensionError if the candidate block is invalid
func (m *ParticipantState) Extend(ctx context.Context, candidate *flow.Block) error {
	span, ctx := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtend)
	defer span.End()

	// check if candidate block has been already processed
	blockID := candidate.ID()
	isDuplicate, err := m.checkBlockAlreadyProcessed(blockID)
	if err != nil || isDuplicate {
		return err
	}

	// check if the block header is a valid extension of parent block
	deferredBlockPersist, err := m.headerExtend(ctx, candidate, nil)
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
	_, deferredSealPersist, err := m.sealExtend(ctx, candidate)
	if err != nil {
		return fmt.Errorf("payload seal(s) not compliant with chain state: %w", err)
	}
	deferredBlockPersist.Chain(deferredSealPersist)

	// evolve protocol state and verify consistency with commitment included in payload
	deferredProtocolStatePersist, err := m.evolveProtocolState(ctx, candidate)
	if err != nil {
		return fmt.Errorf("evolving protocol state failed: %w", err)
	}
	deferredBlockPersist.Chain(deferredProtocolStatePersist)

	// Final step: for atomicity we acquire lock and then execute the deferred operations
	// within the protection of the lock.
	lctx := m.lockManager.NewContext()
	defer lctx.Release()
	err = lctx.AcquireLock(storage.LockInsertBlock)
	if err != nil {
		return err
	}

	// Execute the deferred database operations and emit scheduled notifications on success.
	// The `candidate` block _must be valid_ (otherwise, the state will be corrupted)!
	return m.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return deferredBlockPersist.Execute(lctx, blockID, rw)
	})
}

// headerExtend verifies the validity of the block header (excluding verification of the
// consensus rules). Specifically, we check that
//  1. the payload is consistent with the payload hash stated in the header
//  2. candidate header is consistent with its parent:
//     - ChainID is identical
//     - height increases by 1
//     - ParentView stated by the candidate block equals the parent's actual view
//  3. candidate's block time conforms to protocol rules
//  4. If a `certifyingQC` is given (can be nil), we sanity-check that it certifies the candidate block
//
// If all checks pass, this method queues the following operations to persist the candidate block and
// schedules `BlockProcessable` notification to be emitted in order of increasing height:
//
//	5a. store QC embedded into the candidate block and emit `BlockProcessable` notification for the parent
//	5b. store candidate block and index it as a child of its parent (needed for recovery to traverse unfinalized blocks)
//	5c. if we are given a certifyingQC, store it and queue a `BlockProcessable` notification for the candidate block
//
// If `headerExtend` is called by `ParticipantState.Extend` (full consensus participant) then `certifyingQC` will be nil,
// but the block payload will be validated. If `headerExtend` is called by `FollowerState.Extend` (consensus follower),
// then `certifyingQC` must be not nil which proves payload validity.
//
// Expected errors during normal operations:
//   - state.InvalidExtensionError if the candidate block is invalid
func (m *FollowerState) headerExtend(ctx context.Context, candidate *flow.Block, certifyingQC *flow.QuorumCertificate) (*deferred.DeferredBlockPersist, error) {
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckHeader)
	defer span.End()
	blockID := candidate.ID()
	header := candidate.Header

	// STEP 1: Check that the payload is consistent with the payload hash in the header
	if candidate.Payload.Hash() != header.PayloadHash {
		return nil, state.NewInvalidExtensionErrorf("payload integrity check failed")
	}

	// STEP 2: check whether the candidate (i) connects to the known block tree and
	// (ii) has the same chain ID as its parent and a height incremented by 1.
	parent, err := m.headers.ByBlockID(header.ParentID) // (i) connects to the known block tree
	if err != nil {
		// The only sentinel error that can happen here is `storage.ErrNotFound`. However, by convention the
		// protocol state must be extended in a parent-first order. This block's parent being unknown breaks
		// with this API contract and results in an exception.
		return nil, irrecoverable.NewExceptionf("could not retrieve the candidate's parent block %v: %w", header.ParentID, err)
	}
	if header.ChainID != parent.ChainID {
		return nil, state.NewInvalidExtensionErrorf("candidate built for invalid chain (candidate: %s, parent: %s)",
			header.ChainID, parent.ChainID)
	}
	if header.ParentView != parent.View {
		return nil, state.NewInvalidExtensionErrorf("candidate build with inconsistent parent view (candidate: %d, parent %d)",
			header.ParentView, parent.View)
	}
	if header.Height != parent.Height+1 {
		return nil, state.NewInvalidExtensionErrorf("candidate built with invalid height (candidate: %d, parent: %d)",
			header.Height, parent.Height)
	}

	// STEP 3: check validity of block timestamp using parent's timestamp
	err = m.blockTimer.Validate(parent.Timestamp, header.Timestamp)
	if err != nil {
		if protocol.IsInvalidBlockTimestampError(err) {
			return nil, state.NewInvalidExtensionErrorf("candidate contains invalid timestamp: %w", err)
		}
		return nil, fmt.Errorf("validating block's time stamp failed with unexpected error: %w", err)
	}

	// STEP 4: if a certifying QC is given (can be nil), sanity-check that it actually certifies the candidate block
	if certifyingQC != nil {
		if certifyingQC.View != header.View {
			return nil, fmt.Errorf("qc doesn't certify candidate block, expect %d view, got %d", header.View, certifyingQC.View)
		}
		if certifyingQC.BlockID != blockID {
			return nil, fmt.Errorf("qc doesn't certify candidate block, expect %x blockID, got %x", blockID, certifyingQC.BlockID)
		}
	}

	// STEP 5:
	qc := candidate.Header.ParentQC()
	deferredBlockPersist := deferred.NewDeferredBlockPersist().AddNextOperation(func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
		// STEP 5a: Store QC for parent block and emit `BlockProcessable` notification if and only if
		//  - the QC for the parent has not been stored before (otherwise, we already emitted the notification) and
		//  - the parent block's height is larger than the finalized root height (the root block is already considered processed)
		// Thereby, we reduce duplicated `BlockProcessable` notifications.
		err = m.qcs.BatchStore(lctx, rw, qc)
		if err != nil {
			if !errors.Is(err, storage.ErrAlreadyExists) {
				return fmt.Errorf("could not store incorporated qc: %w", err)
			}
		} else {
			// trigger BlockProcessable for parent block above root height
			if parent.Height > m.finalizedRootHeight {
				storage.OnCommitSucceed(rw, func() {
					m.consumer.BlockProcessable(parent, qc)
				})
			}
		}

		// STEP 5b: Store candidate block and index it as a child of its parent (needed for recovery to traverse unfinalized blocks)
		err = m.blocks.BatchStore(lctx, rw, candidate) // insert the block into the database AND cache
		if err != nil {
			return fmt.Errorf("could not store candidate block: %w", err)
		}
		err = procedure.IndexNewBlock(lctx, rw, blockID, candidate.Header.ParentID)
		if err != nil {
			return fmt.Errorf("could not index new block: %w", err)
		}

		// STEP 5c: if we are given a certifyingQC, store it and queue a `BlockProcessable` notification for the candidate block
		if certifyingQC != nil {
			err = m.qcs.BatchStore(lctx, rw, certifyingQC)
			if err != nil {
				return fmt.Errorf("could not store certifying qc: %w", err)
			}
			storage.OnCommitSucceed(rw, func() { // queue a BlockProcessable event for candidate block, since it is certified
				m.consumer.BlockProcessable(candidate.Header, certifyingQC)
			})
		}
		return nil
	})

	return deferredBlockPersist, nil
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
	err := operation.RetrieveFinalizedHeight(m.db.Reader(), &finalizedHeight)
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = operation.LookupBlockHeight(m.db.Reader(), finalizedHeight, &finalID)
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
// Expected errors during normal operations:
//   - state.InvalidExtensionError if the candidate block contains invalid collection guarantees
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

// sealExtend checks the compliance of the payload seals. It queues a deferred database
// operation for indexing the latest seal as of the candidate block and returns the latest seal.
// Expected errors during normal operations:
//   - state.InvalidExtensionError if the candidate block has invalid seals
func (m *ParticipantState) sealExtend(ctx context.Context, candidate *flow.Block) (*flow.Seal, *deferred.DeferredBlockPersist, error) {
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckSeals)
	defer span.End()

	lastSeal, err := m.sealValidator.Validate(candidate)
	if err != nil {
		return nil, nil, state.NewInvalidExtensionErrorf("seal validation error: %w", err)
	}

	deferredBlockPersist := deferred.NewDeferredBlockPersist().AddNextOperation(
		func(lctx lockctx.Proof, blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
			return operation.IndexLatestSealAtBlock(lctx, rw.Writer(), blockID, lastSeal.ID())
		})

	return lastSeal, deferredBlockPersist, nil
}

// receiptExtend checks the compliance of the receipt payload.
//   - Receipts should pertain to blocks on the fork
//   - Receipts should not appear more than once on a fork
//   - Receipts should pass the ReceiptValidator check
//   - No seal has been included for the respective block in this particular fork
//
// We require the receipts to be sorted by block height (within a payload).
//
// Expected errors during normal operations:
//   - state.InvalidExtensionError if the candidate block contains invalid receipts
func (m *ParticipantState) receiptExtend(ctx context.Context, candidate *flow.Block) error {
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorExtendCheckReceipts)
	defer span.End()

	err := m.receiptValidator.ValidatePayload(candidate)
	if err != nil {
		if engine.IsInvalidInputError(err) {
			return state.NewInvalidExtensionErrorf("payload includes invalid receipts: %w", err)
		}
		if module.IsUnknownBlockError(err) {
			// By convention, the protocol state must be extended in a parent-first order. This block's parent
			// being unknown breaks with this API contract and results in an exception.
			return irrecoverable.NewExceptionf("internal state corruption detected when validating receipts in candidate block %v: %w", candidate.ID(), err)
		}
		return fmt.Errorf("unexpected payload validation error %w", err)
	}
	return nil
}

// lastSealed returns the highest sealed block from the fork with head `candidate`.
//
// For instance, here is the chain state: block 100 is the head, block 97 is finalized,
// and 95 is the last sealed block at the state of block 100.
// 95 (sealed) <- 96 <- 97 (finalized) <- 98 <- 99 <- 100
// Now, if block 101 is extending block 100, and its payload has a seal for 96, then it will
// be the last sealed as of block 101. The result is independent of finalization.
// No errors are expected during normal operation.
func (m *FollowerState) lastSealed(candidate *flow.Block) (latestSeal *flow.Seal, err error) {
	payload := candidate.Payload

	// If the candidate blocks' payload has no seals, the latest seal in this fork remains unchanged, i.e. latest seal as of the
	// parent is also the latest seal as of the candidate block. Otherwise, we take the latest seal included in the candidate block.
	// Note that seals might not be ordered in the block.
	if len(payload.Seals) == 0 {
		latestSeal, err = m.seals.HighestInFork(candidate.Header.ParentID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve parent seal (%x): %w", candidate.Header.ParentID, err)
		}
	} else {
		ordered, err := protocol.OrderedSeals(payload.Seals, m.headers)
		if err != nil {
			// all errors are unexpected - differentiation is for clearer error messages
			if errors.Is(err, storage.ErrNotFound) {
				return nil, irrecoverable.NewExceptionf("ordering seals: candidate payload contains seals for unknown block: %w", err)
			}
			if errors.Is(err, protocol.ErrDiscontinuousSeals) || errors.Is(err, protocol.ErrMultipleSealsForSameHeight) {
				return nil, irrecoverable.NewExceptionf("ordering seals: candidate payload contains invalid seal set: %w", err)
			}
			return nil, fmt.Errorf("unexpected error ordering seals: %w", err)
		}
		latestSeal = ordered[len(ordered)-1]
	}

	return latestSeal, nil
}

// evolveProtocolState
//   - instantiates a Protocol State EvolvingState from the parent block's state
//   - applies any state-changing service events sealed by this block
//   - verifies that the resulting protocol state is consistent with the commitment in the block
//
// Expected errors during normal operations:
//   - state.InvalidExtensionError if the Protocol State commitment in the candidate block does
//     not match the Protocol State we constructed locally
func (m *FollowerState) evolveProtocolState(ctx context.Context, candidate *flow.Block) (*deferred.DeferredBlockPersist, error) {
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorEvolveProtocolState)
	defer span.End()

	// Evolve the Protocol State starting from the parent block's state. Information that may change the state is:
	// the candidate block's view and Service Events from execution results sealed in the candidate block.
	updatedStateID, deferredBlockPersist, err := m.protocolState.EvolveState(candidate.Header.ParentID, candidate.Header.View, candidate.Payload.Seals)
	if err != nil {
		return nil, fmt.Errorf("evolving protocol state failed: %w", err)
	}

	// verify Protocol State commitment in the candidate block matches the locally-constructed value
	if updatedStateID != candidate.Payload.ProtocolStateID {
		return nil, state.NewInvalidExtensionErrorf("invalid protocol state commitment %x in block, which should be %x", candidate.Payload.ProtocolStateID, updatedStateID)
	}

	return deferredBlockPersist, nil
}

// Finalize marks the specified block as finalized.
// This method only finalizes one block at a time.
// Hence, the parent of `blockID` has to be the last finalized block.
// No errors are expected during normal operations.
func (m *FollowerState) Finalize(ctx context.Context, blockID flow.Identifier) error {
	lctx := m.lockManager.NewContext()
	defer lctx.Release()
	err := lctx.AcquireLock(storage.LockFinalizeBlock)
	if err != nil {
		return err
	}

	// preliminaries: start tracer and retrieve full block
	span, _ := m.tracer.StartSpanFromContext(ctx, trace.ProtoStateMutatorFinalize)
	defer span.End()
	block, err := m.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve full block that should be finalized: %w", err)
	}
	header := block.Header

	// keep track of metrics updates and protocol events to emit:
	//  - metrics are updated after a successful database update
	//  - protocol events are emitted atomically with the database update
	var metrics []func()
	var events []func()

	// Verify that the parent block is the latest finalized block.
	// this must be the case, as the `Finalize` method only finalizes one block
	// at a time and hence the parent of `blockID` must already be finalized.
	var finalized uint64
	err = operation.RetrieveFinalizedHeight(m.db.Reader(), &finalized)
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = operation.LookupBlockHeight(m.db.Reader(), finalized, &finalID)
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
	parentEpochState, err := m.protocolState.EpochStateAtBlockID(block.Header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent protocol state snapshot: %w", err)
	}
	finalizingEpochState, err := m.protocolState.EpochStateAtBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve protocol state snapshot: %w", err)
	}
	currentEpochSetup := finalizingEpochState.EpochSetup()

	// Determine metric updates and protocol events related to epoch phase changes and epoch transitions.
	epochPhaseMetrics, epochPhaseEvents, err := m.epochMetricsAndEventsOnBlockFinalized(parentEpochState, finalizingEpochState, header)
	if err != nil {
		return fmt.Errorf("could not determine epoch phase metrics/events for finalized block: %w", err)
	}
	metrics = append(metrics, epochPhaseMetrics...)
	events = append(events, epochPhaseEvents...)

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
	err = m.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		err = operation.IndexBlockHeight(lctx, rw, header.Height, blockID)
		if err != nil {
			return fmt.Errorf("could not insert number mapping: %w", err)
		}
		err = operation.UpsertFinalizedHeight(lctx, rw.Writer(), header.Height)
		if err != nil {
			return fmt.Errorf("could not update finalized height: %w", err)
		}
		err = operation.UpsertSealedHeight(lctx, rw.Writer(), sealed.Height)
		if err != nil {
			return fmt.Errorf("could not update sealed height: %w", err)
		}

		if isFirstBlockOfEpoch(parentEpochState, finalizingEpochState) {
			err = operation.InsertEpochFirstHeight(lctx, rw, currentEpochSetup.Counter, header.Height)
			if err != nil {
				return fmt.Errorf("could not insert epoch first block height: %w", err)
			}
		}

		// When a block is finalized, we commit the result for each seal it contains. The sealing logic
		// guarantees that only a single, continuous execution fork is sealed. Here, we index for
		// each block ID the ID of its _finalized_ seal.
		for _, seal := range block.Payload.Seals {
			err = operation.IndexFinalizedSealByBlockID(rw.Writer(), seal.BlockID, seal.ID())
			if err != nil {
				return fmt.Errorf("could not index the seal by the sealed block ID: %w", err)
			}
		}

		if len(versionBeacons) > 0 {
			// only index the last version beacon as that is the relevant one.
			// TODO: The other version beacons can be used for validation.
			err := operation.IndexVersionBeaconByHeight(rw.Writer(), versionBeacons[len(versionBeacons)-1])
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
	m.State.cachedLatest.Store(&cachedLatest{
		finalizedID:     blockID,
		finalizedHeader: header,
		sealedID:        lastSeal.BlockID,
		sealedHeader:    sealed,
	})

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
	protocolSnapshot, err := m.protocolKVStoreSnapshotsDB.ByID(block.Payload.ProtocolStateID)
	if err != nil {
		return fmt.Errorf("could not retrieve protocol snapshot for block (%x): %w", blockID, err)
	}
	m.metrics.ProtocolStateVersion(protocolSnapshot.Version)

	// apply all queued metrics
	for _, updateMetric := range metrics {
		updateMetric()
	}

	return nil
}

// isFirstBlockOfEpoch returns true if the given block is the first block of a new epoch
// by comparing the block's Protocol State Snapshot to that of its parent.
// NOTE: There can be multiple (un-finalized) blocks that qualify as the first block of epoch N.
func isFirstBlockOfEpoch(parentEpochState, blockEpochState protocol.EpochProtocolState) bool {
	return parentEpochState.Epoch() < blockEpochState.Epoch()
}

// epochMetricsAndEventsOnBlockFinalized determines metrics to update and protocol
// events to emit upon finalizing a block.
//   - We notify about an epoch transition when the first block of the new epoch is finalized
//   - We notify about an epoch phase transition when the first block within the new epoch phase is finalized
//
// This method must be called for each finalized block.
// No errors are expected during normal operation.
func (m *FollowerState) epochMetricsAndEventsOnBlockFinalized(parentEpochState, finalizedEpochState protocol.EpochProtocolState, finalized *flow.Header) (
	metrics []func(),
	events []func(),
	err error,
) {
	parentEpochCounter := parentEpochState.Epoch()
	childEpochCounter := finalizedEpochState.Epoch()
	parentEpochPhase := parentEpochState.EpochPhase()
	childEpochPhase := finalizedEpochState.EpochPhase()

	// Check for entering or exiting EFM
	if !parentEpochState.EpochFallbackTriggered() && finalizedEpochState.EpochFallbackTriggered() {
		// this block triggers EFM
		events = append(events, func() {
			m.consumer.EpochFallbackModeTriggered(childEpochCounter, finalized)
		})
		metrics = append(metrics, m.metrics.EpochFallbackModeTriggered)
	}
	if parentEpochState.EpochFallbackTriggered() && !finalizedEpochState.EpochFallbackTriggered() {
		// this block exits EFM
		events = append(events, func() {
			m.consumer.EpochFallbackModeExited(childEpochCounter, finalized)
		})
		metrics = append(metrics, m.metrics.EpochFallbackModeExited)
	}

	// Check for a new epoch extension
	if len(finalizedEpochState.EpochExtensions()) > len(parentEpochState.EpochExtensions()) {
		// We expect at most one additional epoch extension per block, but tolerate more here
		for i := len(parentEpochState.EpochExtensions()); i < len(finalizedEpochState.EpochExtensions()); i++ {
			finalizedExtension := finalizedEpochState.EpochExtensions()[i]
			events = append(events, func() { m.consumer.EpochExtended(childEpochCounter, finalized, finalizedExtension) })
			metrics = append(metrics, func() { m.metrics.CurrentEpochFinalView(finalizedExtension.FinalView) })
		}
	}

	// Different epoch counter - handle epoch transition and phase transition Committed->Staking
	if parentEpochCounter != childEpochCounter {
		childEpochSetup := finalizedEpochState.EpochSetup()
		events = append(events, func() { m.consumer.EpochTransition(childEpochSetup.Counter, finalized) })
		// set current epoch counter corresponding to new epoch
		metrics = append(metrics, func() { m.metrics.CurrentEpochCounter(childEpochSetup.Counter) })
		// denote the most recent epoch transition height
		metrics = append(metrics, func() { m.metrics.EpochTransitionHeight(finalized.Height) })
		// set epoch phase
		metrics = append(metrics, func() { m.metrics.CurrentEpochPhase(childEpochPhase) })
		// set current epoch view values
		metrics = append(
			metrics,
			// Since we have just started a new epoch, there cannot be any extensions yet.
			// Therefore, it is safe to directly use EpochSetup.FinalView here (epoch extensions are handled above).
			func() { m.metrics.CurrentEpochFinalView(childEpochSetup.FinalView) },
			func() {
				m.metrics.CurrentDKGPhaseViews(childEpochSetup.DKGPhase1FinalView, childEpochSetup.DKGPhase2FinalView, childEpochSetup.DKGPhase3FinalView)
			},
		)
		return
	}

	// Same epoch phase -> nothing to do
	if parentEpochPhase == childEpochPhase {
		return
	}

	// Update the phase metric when any phase change occurs
	events = append(events, func() { m.metrics.CurrentEpochPhase(childEpochPhase) })

	// Handle phase transition Staking->Setup. `finalized` is first block in Setup phase.
	if parentEpochPhase == flow.EpochPhaseStaking && childEpochPhase == flow.EpochPhaseSetup {
		events = append(events, func() { m.consumer.EpochSetupPhaseStarted(childEpochCounter, finalized) })
		return
	}
	// Handle phase transition Setup/Fallback->Committed phase. `finalized` is first block in Committed phase.
	if (parentEpochPhase == flow.EpochPhaseSetup || parentEpochPhase == flow.EpochPhaseFallback) && childEpochPhase == flow.EpochPhaseCommitted {
		events = append(events, func() { m.consumer.EpochCommittedPhaseStarted(childEpochCounter, finalized) })
		return
	}
	// Handle phase transition Staking/Setup->Fallback phase
	// NOTE: we can have the phase transition Committed->Fallback, but only across an epoch boundary (handled above)
	if (parentEpochPhase == flow.EpochPhaseStaking || parentEpochPhase == flow.EpochPhaseSetup) && childEpochPhase == flow.EpochPhaseFallback {
		// This conditional exists to capture this final set of valid phase transitions, to allow sanity check below
		// In the future we could add a protocol event here for transition into the Fallback phase, if any consumers need this.
		return
	}

	return nil, nil, fmt.Errorf("sanity check failed: invalid subsequent [epoch-phase] [%d-%s]->[%d-%s]",
		parentEpochCounter, parentEpochPhase, childEpochCounter, childEpochPhase)
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
