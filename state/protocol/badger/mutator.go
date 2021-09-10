// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
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

// errIncompleteEpochConfiguration is a sentinel error returned when there are
// still epoch service events missing and the new epoch can't be constructed.
var errIncompleteEpochConfiguration = errors.New("block beyond epoch boundary")

// FollowerState implements a lighter version of a mutable protocol state.
// When extending the state, it performs hardly any checks on the block payload.
// Instead, the FollowerState relies on the consensus nodes to run the full
// payload check. Consequently, a block B should only be considered valid, if
// a child block with a valid header is known. The child block's header
// includes quorum certificate, which proves that a supermajority of consensus
// nodes consider block B a valid.
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
func (m *FollowerState) Extend(candidate *flow.Block) error {

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorHeaderExtend)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorHeaderExtend)

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
	err = m.insert(candidate, last)
	if err != nil {
		return fmt.Errorf("failed to insert the block: %w", err)
	}

	return nil
}

// Extend extends the protocol state of a CONSENSUS PARTICIPANT. It checks
// the validity of the _entire block_ (header and full payload).
func (m *MutableState) Extend(candidate *flow.Block) error {

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtend)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtend)

	// check if the block header is a valid extension of the finalized state
	err := m.headerExtend(candidate)
	if err != nil {
		return fmt.Errorf("header does not compliance the chain state: %w", err)
	}

	// check if the guarantees in the payload is a valid extension of the finalized state
	err = m.guaranteeExtend(candidate)
	if err != nil {
		return fmt.Errorf("guarantee does not compliance the chain state: %w", err)
	}

	// check if the receipts in the payload are valid
	err = m.receiptExtend(candidate)
	if err != nil {
		return fmt.Errorf("payload receipts not compliant with chain state: %w", err)
	}

	// check if the seals in the payload is a valid extension of the finalized
	// state
	lastSeal, err := m.sealExtend(candidate)
	if err != nil {
		return fmt.Errorf("seal in parent block does not compliance the chain state: %w", err)
	}

	// insert the block and index the last seal for the block
	err = m.insert(candidate, lastSeal)
	if err != nil {
		return fmt.Errorf("failed to insert the block: %w", err)
	}

	return nil
}

// headerExtend verifies the validity of the block header (excluding verification of the
// consensus rules). Specifically, we check that the block connects to the last finalized block.
func (m *FollowerState) headerExtend(candidate *flow.Block) error {

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtendCheckHeader)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtendCheckHeader)

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
func (m *MutableState) guaranteeExtend(candidate *flow.Block) error {

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtendCheckGuarantees)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtendCheckGuarantees)

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
func (m *MutableState) sealExtend(candidate *flow.Block) (*flow.Seal, error) {
	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtendCheckSeals)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtendCheckSeals)

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
func (m *MutableState) receiptExtend(candidate *flow.Block) error {
	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtendCheckReceipts)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtendCheckReceipts)

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

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorHeaderExtendGetLastSealed)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorHeaderExtendGetLastSealed)

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

// insert stores the candidate block in the data base. The
// `candidate` block _must be valid_ (otherwise, the state will be corrupted).
func (m *FollowerState) insert(candidate *flow.Block, last *flow.Seal) error {

	blockID := candidate.ID()

	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtendDBInsert)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtendDBInsert)

	// SIXTH: epoch transitions and service events
	//    (i) Determine protocol state for block's _current_ Epoch.
	//        As we don't have slashing yet, the protocol state is fully
	//        determined by the Epoch Preparation events.
	//   (ii) Determine protocol state for block's _next_ Epoch.
	//        In case any of the payload seals includes system events,
	//        we need to check if they are valid and must apply them
	//        to the protocol state as needed.
	ops, err := m.handleServiceEvents(candidate)
	if err != nil {
		return fmt.Errorf("could not handle service events: %w", err)
	}

	// FINALLY: Both the header itself and its payload are in compliance with the
	// protocol state. We can now store the candidate block, as well as adding
	// its final seal to the seal index and initializing its children index.

	err = operation.RetryOnConflictTx(m.db, transaction.Update, func(tx *transaction.Tx) error {
		// insert the block into the database AND cache
		err := m.blocks.StoreTx(candidate)(tx)
		if err != nil {
			return fmt.Errorf("could not store candidate block: %w", err)
		}

		// index the latest sealed block in this fork
		err = transaction.WithTx(operation.IndexBlockSeal(blockID, last.ID()))(tx)
		if err != nil {
			return fmt.Errorf("could not index candidate seal: %w", err)
		}

		// index the child block for recovery
		err = transaction.WithTx(procedure.IndexNewBlock(blockID, candidate.Header.ParentID))(tx)
		if err != nil {
			return fmt.Errorf("could not index new block: %w", err)
		}

		// apply any optional DB operations from service events
		for _, apply := range ops {
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

	return nil
}

// Finalize marks the specified block as finalized. This method only
// finalizes one block at a time. Hence, the parent of `blockID`
// has to be the last finalized block.
func (m *FollowerState) Finalize(blockID flow.Identifier) error {
	// preliminaries: start tracer and retrieve full block
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorFinalize)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorFinalize)
	block, err := m.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve full block that should be finalized: %w", err)
	}
	header := block.Header

	// FIRST: verify that the parent block is the latest finalized block. This
	// must be the case, as the `Finalize(..)` method only finalizes one block
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

	// SECOND: We also want to update the last sealed height. Retrieve the block
	// seal indexed for the block and retrieve the block that was sealed by it.
	last, err := m.seals.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not look up sealed header: %w", err)
	}
	sealed, err := m.headers.ByBlockID(last.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed header: %w", err)
	}

	// THIRD: preparing Epoch-Phase-Change service notifications and metrics updates.
	// Convention:
	//                            .. <--- P <----- B
	//                                    ↑        ↑
	//             block sealing service event        first block of new
	//           for epoch-phase transition        Epoch phase (e.g.
	//              (e.g. EpochSetup event)        (EpochSetup phase)
	// Per convention, service notifications for Epoch-Phase-Changes are emitted, when
	// the first block of the new phase (EpochSetup phase) is _finalized_. Meaning
	// that the new phase has started.
	epochStatus, err := m.epoch.statuses.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve epoch state: %w", err)
	}
	currentEpochSetup, err := m.epoch.setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		return fmt.Errorf("could not retrieve setup event for current epoch: %w", err)
	}
	parent, err := m.blocks.ByID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not get parent (id=%x): %w", header.ParentID, err)
	}

	// track service event driven metrics and protocol events that should be emitted
	var events []func()
	for _, seal := range parent.Payload.Seals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return fmt.Errorf("could not retrieve result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}
		for _, event := range result.ServiceEvents {
			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				// update current epoch phase
				events = append(events, func() { m.metrics.CurrentEpochPhase(flow.EpochPhaseSetup) })
				// track epoch phase transition (staking->setup)
				events = append(events, func() { m.consumer.EpochSetupPhaseStarted(ev.Counter-1, header) })
			case *flow.EpochCommit:
				// update current epoch phase
				events = append(events, func() { m.metrics.CurrentEpochPhase(flow.EpochPhaseCommitted) })
				// track epoch phase transition (setup->committed)
				events = append(events, func() { m.consumer.EpochCommittedPhaseStarted(ev.Counter-1, header) })
				// track final view of committed epoch
				nextEpochSetup, err := m.epoch.setups.ByID(epochStatus.NextEpoch.SetupID)
				if err != nil {
					return fmt.Errorf("could not retrieve setup event for next epoch: %w", err)
				}
				events = append(events, func() { m.metrics.CommittedEpochFinalView(nextEpochSetup.FinalView) })
			default:
				return fmt.Errorf("invalid service event type in payload (%T)", event)
			}
		}
	}

	// FOURTH: preparing Epoch-Change service notifications and metrics updates.
	// Convention:
	// Service notifications and updating metrics happen when we finalize the _first_
	// block of the new Epoch (same convention as for Epoch-Phase-Changes)
	// Approach: We retrieve the parent block's epoch information. If this block's view
	// exceeds the final view of its parent's current epoch, this block begins the next epoch.
	parentBlocksEpoch := m.AtBlockID(header.ParentID).Epochs().Current()
	parentEpochFinalView, err := parentBlocksEpoch.FinalView()
	if err != nil {
		return fmt.Errorf("could not get parent epoch final view: %w", err)
	}

	if header.View > parentEpochFinalView {
		// TMP: EMERGENCY EPOCH CHAIN CONTINUATION [EECC]
		//
		// If we have triggered emergency chain continuation as a result of a
		// failed epoch, these events would be emitted for every block. Instead,
		// we will skip them.
		//
		// We detect EECC here by checking for two blocks spanning what should
		// be an epoch transition having the same epoch counter. This indicates
		// that the last epoch was continued past its specified end time.
		parentCounter, err := parentBlocksEpoch.Counter()
		if err != nil {
			return fmt.Errorf("could not check parent counter to skip events in fallback epoch: %w", err)
		}
		if parentCounter != currentEpochSetup.Counter {
			events = append(events, func() { m.consumer.EpochTransition(currentEpochSetup.Counter, header) })

			// set current epoch counter corresponding to new epoch
			events = append(events, func() { m.metrics.CurrentEpochCounter(currentEpochSetup.Counter) })
			// set epoch phase - since we are starting a new epoch we begin in the staking phase
			events = append(events, func() { m.metrics.CurrentEpochPhase(flow.EpochPhaseStaking) })
		}
	}

	// FIFTH: Persist updates in data base
	// * Add this block to the height-indexed set of finalized blocks.
	// * Update the largest finalized height to this block's height.
	// * Update the largest height of sealed and finalized block.
	//   This value could actually stay the same if it has no seals in
	//   its payload, in which case the parent's seal is the same.
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
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not execute finalization: %w", err)
	}

	// FINALLY: emit notification events and update metrics
	m.metrics.FinalizedHeight(header.Height)
	m.metrics.SealedHeight(sealed.Height)
	m.metrics.BlockFinalized(block)
	m.consumer.BlockFinalized(header)
	for _, emit := range events {
		emit()
	}
	for _, seal := range block.Payload.Seals {
		sealedBlock, err := m.blocks.ByID(seal.BlockID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed block (%x): %w", seal.BlockID, err)
		}
		m.metrics.BlockSealed(sealedBlock)
	}

	return nil
}

// epochStatus computes the EpochStatus for the given block
// BEFORE applying the block payload itself
// Specifically, we must determine whether block is the first block of a new
// epoch in its respective fork. We do this by comparing the block's view to
// the Epoch data from its parent. If the block's view is _larger_ than the
// final View of the parent's epoch, the block starts a new Epoch.
// case (a): block is in same Epoch as parent.
//           the parent's EpochStatus.CurrentEpoch also applies for the current block
// case (b): block starts new Epoch in its respective fork.
//           the parent's EpochStatus.NextEpoch is the current block's EpochStatus.CurrentEpoch
// As the parent was a valid extension of the chain, by induction, the parent satisfies all
// consistency requirements of the protocol.
//
// Returns:
// * errIncompleteEpochConfiguration if the epoch has ended before processing
//   both an EpochSetup and EpochCommit event; so the new epoch can't be constructed.
func (m *FollowerState) epochStatus(block *flow.Header) (*flow.EpochStatus, error) {

	parentStatus, err := m.epoch.statuses.ByBlockID(block.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch state for parent: %w", err)
	}

	// Retrieve EpochSetup and EpochCommit event for parent block's Epoch
	parentSetup, err := m.epoch.setups.ByID(parentStatus.CurrentEpoch.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve EpochSetup event for parent: %w", err)
	}

	if parentSetup.FinalView < block.View { // first block of a new epoch
		// sanity check: parent's epoch Preparation should be completed and have EpochSetup and EpochCommit events
		if parentStatus.NextEpoch.SetupID == flow.ZeroID {
			return nil, fmt.Errorf("missing setup event for starting next epoch: %w", errIncompleteEpochConfiguration)
		}
		if parentStatus.NextEpoch.CommitID == flow.ZeroID {
			return nil, fmt.Errorf("missing commit event for starting next epoch: %w", errIncompleteEpochConfiguration)
		}
		status, err := flow.NewEpochStatus(
			parentStatus.CurrentEpoch.SetupID, parentStatus.CurrentEpoch.CommitID,
			parentStatus.NextEpoch.SetupID, parentStatus.NextEpoch.CommitID,
			flow.ZeroID, flow.ZeroID,
		)
		return status, err
	}

	// Block is in the same epoch as its parent, re-use the same epoch status
	// IMPORTANT: copy the status to avoid modifying the parent status in the cache
	currentStatus := parentStatus.Copy()
	return currentStatus, err
}

// handleServiceEvents handles applying state changes which occur as a result
// of service events being included in a block payload.
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
// If the service events are valid, or there are no service events, this method
// returns a slice of Badger operations to apply while storing the block. This
// includes an operation to index the epoch status for every block, and
// operations to insert service events for blocks that include them.
//
// Return values:
//  * ops: pending data base operations to persist this processing step
//  * error: no errors expected during normal operations
func (m *FollowerState) handleServiceEvents(block *flow.Block) ([]func(*transaction.Tx) error, error) {
	var ops []func(*transaction.Tx) error

	// Determine epoch status for block's CURRENT epoch.
	//
	// This yields the tentative protocol state BEFORE applying the block payload.
	// As we don't have slashing yet, there is nothing in the payload which could
	// modify the protocol state for the current epoch.

	epochStatus, err := m.epochStatus(block.Header)
	if errors.Is(err, errIncompleteEpochConfiguration) {
		// TMP: EMERGENCY EPOCH CHAIN CONTINUATION
		//
		// We are proposing or processing the first block of the next epoch,
		// but that epoch has not been setup. Rather than returning an error
		// which prevents further block production, we store the block with
		// the same epoch status as its parent, resulting in it being considered
		// by the protocol state to fall in the same epoch as its parent.
		//
		// CAUTION: this is inconsistent with the FinalView value specified in the epoch.
		fmt.Printf("handleServiceEvents: emergency epoch chain continuation triggered at block id: %x, height: %d\n", block.ID(), block.Header.Height)
		parentStatus, err := m.epoch.statuses.ByBlockID(block.Header.ParentID)
		if err != nil {
			return nil, fmt.Errorf("internal error constructing EECC from parent's epoch status: %w", err)
		}
		ops = append(ops, m.epoch.statuses.StoreTx(block.ID(), parentStatus.Copy()))
		return ops, nil
	} else if err != nil {
		return nil, fmt.Errorf("could not determine epoch status: %w", err)
	}

	activeSetup, err := m.epoch.setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch setup event: %w", err)
	}
	counter := activeSetup.Counter

	// we will apply service events from blocks which are sealed by this block's PARENT
	parent, err := m.blocks.ByID(block.Header.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get parent (id=%x): %w", block.Header.ParentID, err)
	}

	// The payload might contain epoch preparation service events for the next
	// epoch. In this case, we need to update the tentative protocol state.
	// We need to validate whether all information is available in the protocol
	// state to go to the next epoch when needed. In cases where there is a bug
	// in the smart contract, it could be that this happens too late and the
	// chain finalization should halt.
	for _, seal := range parent.Payload.Seals {
		result, err := m.results.ByID(seal.ResultID)
		if err != nil {
			return nil, fmt.Errorf("could not get result (id=%x) for seal (id=%x): %w", seal.ResultID, seal.ID(), err)
		}

		for _, event := range result.ServiceEvents {

			switch ev := event.Event.(type) {
			case *flow.EpochSetup:

				// We should only have a single epoch setup event per epoch.
				if epochStatus.NextEpoch.SetupID != flow.ZeroID {
					// true iff EpochSetup event for NEXT epoch was already included before
					return nil, state.NewInvalidExtensionError("duplicate epoch setup service event")
				}

				// The setup event should have the counter increased by one.
				if ev.Counter != counter+1 {
					return nil, state.NewInvalidExtensionErrorf("next epoch setup has invalid counter (%d => %d)", counter, ev.Counter)
				}

				// The first view needs to be exactly one greater than the current epoch final view
				if ev.FirstView != activeSetup.FinalView+1 {
					return nil, state.NewInvalidExtensionErrorf(
						"next epoch first view must be exactly 1 more than current epoch final view (%d != %d+1)",
						ev.FirstView,
						activeSetup.FinalView,
					)
				}

				// Finally, the epoch setup event must contain all necessary information.
				err = isValidEpochSetup(ev, false)
				if err != nil {
					return nil, state.NewInvalidExtensionErrorf("invalid epoch setup: %s", err)
				}

				// prevents multiple setup events for same Epoch (including multiple setup events in payload of same block)
				epochStatus.NextEpoch.SetupID = ev.ID()

				// we'll insert the setup event when we insert the block
				ops = append(ops, m.epoch.setups.StoreTx(ev))

			case *flow.EpochCommit:

				// We should only have a single epoch commit event per epoch.
				if epochStatus.NextEpoch.CommitID != flow.ZeroID {
					// true iff EpochCommit event for NEXT epoch was already included before
					return nil, state.NewInvalidExtensionError("duplicate epoch commit service event")
				}

				// The epoch setup event needs to happen before the commit.
				if epochStatus.NextEpoch.SetupID == flow.ZeroID {
					return nil, state.NewInvalidExtensionError("missing epoch setup for epoch commit")
				}

				// The commit event should have the counter increased by one.
				if ev.Counter != counter+1 {
					return nil, state.NewInvalidExtensionErrorf("next epoch commit has invalid counter (%d => %d)", counter, ev.Counter)
				}

				// Finally, the commit should commit all the necessary information.
				setup, err := m.epoch.setups.ByID(epochStatus.NextEpoch.SetupID)
				if err != nil {
					return nil, state.NewInvalidExtensionErrorf("could not retrieve next epoch setup: %s", err)
				}
				err = isValidEpochCommit(ev, setup)
				if err != nil {
					return nil, state.NewInvalidExtensionErrorf("invalid epoch commit: %s", err)
				}

				// prevents multiple setup events for same Epoch (including multiple setup events in payload of same block)
				epochStatus.NextEpoch.CommitID = ev.ID()

				// we'll insert the commit event when we insert the block
				ops = append(ops, m.epoch.commits.StoreTx(ev))

			default:
				return nil, fmt.Errorf("invalid service event type: %s", event.Type)
			}
		}
	}

	// we always index the epoch status, even when there are no service events
	ops = append(ops, m.epoch.statuses.StoreTx(block.ID(), epochStatus))

	return ops, nil
}

// MarkValid marks the block as valid in protocol state, and triggers
// `BlockProcessable` event to notify that its parent block is processable.
// why the parent block is processable, not the block itself?
// because a block having a child block means it has been verified
// by the majority of consensus participants.
// Hence, if a block has passed the header validity check, its parent block
// must have passed both the header validity check and the body validity check.
// So that consensus followers can skip the block body validity checks and wait
// for its child to arrive, and if the child passes the header validity check, it means
// the consensus participants have done a complete check on its parent block,
// so consensus followers can trust consensus nodes did the right job, and start
// processing the parent block.
// NOTE: since a parent can have multiple children, `BlockProcessable` event
// could be triggered multiple times for the same block.
// NOTE: BlockProcessable should not be blocking, otherwise, it will block the follower
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

	err = operation.RetryOnConflict(
		m.db.Update,
		operation.SkipDuplicates(
			operation.InsertBlockValidity(blockID, true),
		),
	)
	if err != nil {
		return fmt.Errorf("could not mark block as valid (%x): %w", blockID, err)
	}

	// root blocks and blocks below the root block are considered as "processed",
	// so we don't want to trigger `BlockProcessable` event for them.
	parent, err := m.headers.ByBlockID(parentID)
	if err != nil {
		return fmt.Errorf("could not retrieve block header for %x: %w", parentID, err)
	}
	var rootHeight uint64
	err = m.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return fmt.Errorf("could not retrieve root block's height: %w", err)
	}
	if rootHeight >= parent.Height {
		return nil
	}
	m.consumer.BlockProcessable(parent)

	return nil
}
