// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/validation"
	"github.com/onflow/flow-go/state/protocol"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

type FollowerState struct {
	State
	index    storage.Index
	payloads storage.Payloads
	consumer protocol.Consumer
}

type MutableState struct {
	FollowerState
	validator module.ReceiptValidator
}

// NewState initializes a new state backed by a badger database, applying the
// optional configuration parameters.
func NewFollowerState(
	metrics module.ComplianceMetrics,
	tracer module.Tracer,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	payloads storage.Payloads,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
	consumer protocol.Consumer,
) (*FollowerState, error) {
	partial, err := NewState(metrics, tracer, db, headers, seals, blocks,
		setups, commits, statuses)

	if err != nil {
		return nil, err
	}

	followerState := &FollowerState{
		State:    *partial,
		index:    index,
		payloads: payloads,
		consumer: consumer,
	}
	return followerState, nil
}

func NewMutableState(
	metrics module.ComplianceMetrics,
	tracer module.Tracer,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	payloads storage.Payloads,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
	consumer protocol.Consumer,
	results storage.ExecutionResults,
) (*MutableState, error) {
	partial, err := NewFollowerState(metrics, tracer, db, headers, seals, index, payloads,
		blocks, setups, commits, statuses, consumer)

	if err != nil {
		return nil, err
	}

	signatureVerifier := signature.NewAggregationVerifier(encoding.ExecutionReceiptTag)

	mutableState := &MutableState{
		FollowerState: *partial,
		validator:     validation.NewReceiptValidator(partial, partial.index, results, signatureVerifier),
	}

	return mutableState, nil
}

func (m *FollowerState) Extend(candidate *flow.Block) error {

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorHeaderExtend)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorHeaderExtend)

	// check if he block header is a valid extension of the finalized state
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

func (m *MutableState) Extend(candidate *flow.Block) error {

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtend)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtend)

	// check if the block header is a valid extension of the finalized state
	err := m.headerExtend(candidate)
	if err != nil {
		return fmt.Errorf("header does not compliance the chain state: %w", err)
	}

	// Get the latest seal in the fork that ends with the candidate's parent.
	// The protocol state saves this information for each block that has been
	// successfully added to the chain tree (even when the added block does not
	// itself contain a seal). We just called `headerExtend` to check that the
	// candidate block's header is a valid extension of the chain, which implies
	// that the parent must already be part of the chain tree. Therefore, _not_
	// finding the latest sealed block in the fork up to the parent constitutes
	// a fatal internal error.
	lastSealUpToParent, err := m.seals.ByBlockID(candidate.Header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent seal (%x): %w", candidate.Header.ParentID, err)
	}

	// check if the guarantees in the payload is a valid extension of the finalized state
	err = m.guaranteeExtend(candidate)
	if err != nil {
		return fmt.Errorf("guarantee does not compliance the chain state: %w", err)
	}

	// check if the seals in the payload is a valid extension of the finalized
	// state, return the last seal at the candidate block
	last, err := m.sealExtend(candidate, lastSealUpToParent)
	if err != nil {
		return fmt.Errorf("seal in parent block does not compliance the chain state: %w", err)
	}

	// check if the receipts in the payload are valid
	err = m.receiptExtend(candidate)
	if err != nil {
		return fmt.Errorf("payload receipts not compliant with chain state: %w", err)
	}

	// insert the block and index the last seal for the block
	err = m.insert(candidate, last)
	if err != nil {
		return fmt.Errorf("failed to insert the block: %w", err)
	}
	return nil
}

// header compliance check to verify if the given block connects to the
// last finalized block.
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

// The guarantee part of the payload compliance check.
// None of the blocks should have included a
// guarantee that was expired at the block height, nor should it have been
// included in any previous payload.
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

// sealExtend checks the compliance of the payload seals and returns the last
// valid seal on the fork up to and including `candidate`. Payload seals should
// form a valid chain on top of the last seal as of the parent of `candidate`,
// and should only correspond to blocks and execution results incorporated on
// the current fork.
//
// Note that we don't explicitly check that sealed results satisfy the sub-graph
// check. Nevertheless, correctness in this regard is guaranteed because:
//  * We only allow seals that correspond to ExecutionReceipts that were
//    incorporated in this fork.
//  * We only include ExecutionReceipts whose results pass the sub-graph check
//    (as part of ReceiptValidator).
// => Therefore, only seals whose results pass the sub-graph check will be
//    allowed.
func (m *MutableState) sealExtend(candidate *flow.Block, lastSealUpToParent *flow.Seal) (*flow.Seal, error) {

	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtendCheckSeals)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtendCheckSeals)

	header := candidate.Header
	payload := candidate.Payload

	last := lastSealUpToParent

	// if there is no seal in the block payload, use the last sealed block of
	// the parent block as the last sealed block of the given block.
	if len(payload.Seals) == 0 {
		return last, nil
	}

	// map each seal to the block it is sealing for easy lookup; we will need to
	// successfully connect _all_ of these seals to the last sealed block for
	// the payload to be valid
	byBlock := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range payload.Seals {
		byBlock[seal.BlockID] = seal
	}
	if len(payload.Seals) != len(byBlock) {
		return nil, state.NewInvalidExtensionErrorf("multiple seals for the same block")
	}

	// incorporatedResults collects the _first_ appearance of unsealed execution
	// results on the fork, along with the ID of the block in which they are
	// incorporated.
	incorporatedResults := make(map[flow.Identifier]*flow.IncorporatedResult)

	// collect IDs of blocks on the fork (from parent to last sealed)
	var blockIDs []flow.Identifier

	// loop through the fork backwards up to last sealed block and collect
	// IncorporatedResults as well as the IDs of blocks visited
	sealedID := last.BlockID
	ancestorID := header.ParentID
	for ancestorID != sealedID {

		ancestor, err := m.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve ancestor header (%x): %w", ancestorID, err)
		}

		// keep track of blocks on the fork
		blockIDs = append(blockIDs, ancestorID)

		payload, err := m.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get block payload %x: %w", ancestorID, err)
		}

		// Collect execution results from receipts.
		for _, receipt := range payload.Receipts {
			resultID := receipt.ExecutionResult.ID()
			// incorporatedResults should contain the _first_ appearance of the
			// ExecutionResult. We are traversing the fork backwards, so we can
			// overwrite any previously recorded result.

			// ATTENTION:
			// Here, IncorporatedBlockID (the first argument) should be set
			// to ancestorID, because that is the block that contains the
			// ExecutionResult. However, in phase 2 of the sealing roadmap,
			// we are still using a temporary sealing logic where the
			// IncorporatedBlockID is expected to be the result's block ID.
			incorporatedResults[resultID] = flow.NewIncorporatedResult(
				receipt.ExecutionResult.BlockID,
				&receipt.ExecutionResult,
			)

		}

		ancestorID = ancestor.ParentID
	}

	// Loop forward across the fork and try to create a valid chain of seals.
	// blockIDs, as populated by the previous loop, is in reverse order.
	for i := len(blockIDs) - 1; i >= 0; i-- {
		// if there are no more seals left, we can exit earlier
		if len(byBlock) == 0 {
			return last, nil
		}

		// return an error if there are still seals to consider, but they break
		// the chain
		blockID := blockIDs[i]
		seal, found := byBlock[blockID]
		if !found {
			return nil, state.NewInvalidExtensionErrorf("chain of seals broken (missing: %x)", blockID)
		}

		delete(byBlock, blockID)

		// check if we have an incorporatedResult for this seal
		incorporatedResult, ok := incorporatedResults[seal.ResultID]
		if !ok {
			return nil, state.NewInvalidExtensionErrorf("seal %x does not correspond to a result on this fork", seal.ID())
		}

		// check the integrity of the seal
		valid, err := IsValidSeal(seal, incorporatedResult)
		if err != nil {
			return nil, fmt.Errorf("could not check validity of Seal %v: %w", seal.ID(), err)
		}
		if !valid {
			return nil, state.NewInvalidExtensionErrorf("payload includes invalid seal (%x)", seal.ID())
		}

		last = seal
	}

	// at this point no seals should be left
	if len(byBlock) > 0 {
		return nil, state.NewInvalidExtensionErrorf("not all seals connected to state (left: %d)", len(byBlock))
	}

	return last, nil
}

// IsValidSeal checks the crytographic integrity of the seal and checks that
// the seal has collected enough approval signatures based on the chunk
// assignment of the corresponding incorporated result.
func IsValidSeal(seal *flow.Seal, incorporatedResult *flow.IncorporatedResult) (bool, error) {
	// TODO
	// Check cryptographic integrity
	// Check the aggregated signatures against set of assigned verifiers
	// How do we get the ChunkAssigner?
	return true, nil
}

// receiptExtend checks the compliance of the receipt payload.
//   * Receipts should pertain to blocks on the fork
//   * Receipts should not appear more than once on a fork
//   * No seal has been included for the respective block in this particular fork
// We require the receipts to be sorted by block height (within a payload).
func (m *MutableState) receiptExtend(candidate *flow.Block) error {
	blockID := candidate.ID()
	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorExtendCheckReceipts)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorExtendCheckReceipts)

	header := candidate.Header
	payload := candidate.Payload

	// Get the latest sealed block on this fork, ie the highest block for which
	// there is a seal in this fork. This block is not necessarily finalized.
	last, err := m.seals.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent seal (%x): %w", header.ParentID, err)
	}
	sealed, err := m.headers.ByBlockID(last.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed block (%x): %w", last.BlockID, err)
	}
	sealedHeight := sealed.Height

	// forkBlocks is used to keep the IDs of the blocks we iterate through. We
	// use it to identify receipts that are for blocks not in the fork.
	forkBlocks := make(map[flow.Identifier]*flow.Header)

	// Create a lookup table of all the receipts that are already in blocks.
	// This will be used to identify duplicate receipts.
	lookup := make(map[flow.Identifier]struct{})

	// loop through the fork backwards, from parent to last sealed, and keep
	// track of blocks and receipts visited on the way.
	ancestorID := header.ParentID
	for {

		ancestor, err := m.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor header (%x): %w", ancestorID, err)
		}

		// break out when we reach the sealed height
		if ancestor.Height <= sealedHeight {
			break
		}

		// keep track of blocks we iterate over
		forkBlocks[ancestorID] = ancestor

		index, err := m.index.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor index (%x): %w", ancestorID, err)
		}

		// keep track of all receipts we iterate over
		for _, recID := range index.ReceiptIDs {
			lookup[recID] = struct{}{}
		}

		ancestorID = ancestor.ParentID
	}

	// prevReceiptHeight is used to check that receipts within a payload are
	// sorted by block height. For each receipt in the payload, we check that
	// its block height is greater or equal than the previous receipt.
	var prevReceiptHeight uint64 = 0

	// check each receipt included in the payload for duplication
	for _, receipt := range payload.Receipts {

		// if the receipt was already included before, error
		_, duplicated := lookup[receipt.ID()]
		if duplicated {
			return state.NewInvalidExtensionErrorf("payload includes duplicate receipt (%x)", receipt.ID())
		}

		// if the receipt is not for a block on this fork, error
		header, ok := forkBlocks[receipt.ExecutionResult.BlockID]
		if !ok {
			return state.NewInvalidExtensionErrorf("payload includes receipt for block not on fork (%x)", receipt.ExecutionResult.BlockID)
		}

		err = m.validator.Validate(receipt)
		if err != nil {
			// TODO: this might be not an error, potentially it can be solved by requesting more data and processing this receipt again
			if errors.Is(err, storage.ErrNotFound) {
				return state.NewInvalidExtensionErrorf("some entities referenced by receipt %v are missing: %w", receipt.ID(), err)
			}

			if engine.IsInvalidInputError(err) {
				return state.NewInvalidExtensionErrorf("payload includes invalid receipt %v: %w", receipt.ID(), err)
			}

			return fmt.Errorf("payload validation unexpected error %w", err)
		}

		// check receipts are sorted by block height
		if header.Height < prevReceiptHeight {
			return state.NewInvalidExtensionError("payload receipts should be sorted by block height")
		}

		prevReceiptHeight = header.Height
	}

	return nil
}

// finding the last sealed block on the chain of which the given block is extending
// for instance, here is the chain state: block 100 is the head, block 97 is finalized,
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
				return nil, fmt.Errorf("could not retrieve the header %v for seal: %w", seal.BlockID, err)
			}

			if i == 0 || header.Height > highestHeader.Height {
				highestHeader = header
				last = seal
			}
		}
	}

	return last, nil
}

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

	err = operation.RetryOnConflict(m.db.Update, func(tx *badger.Txn) error {
		// insert the block into the database AND cache
		err := m.blocks.StoreTx(candidate)(tx)
		if err != nil {
			return fmt.Errorf("could not store candidate block: %w", err)
		}

		// index the latest sealed block in this fork
		err = operation.IndexBlockSeal(blockID, last.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index candidate seal: %w", err)
		}

		// index the child block for recovery
		err = procedure.IndexNewBlock(blockID, candidate.Header.ParentID)(tx)
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

func (m *FollowerState) Finalize(blockID flow.Identifier) error {

	m.tracer.StartSpan(blockID, trace.ProtoStateMutatorFinalize)
	defer m.tracer.FinishSpan(blockID, trace.ProtoStateMutatorFinalize)

	// FIRST: The finalize call on the protocol state can only finalize one
	// block at a time. This implies that the parent of the pending block that
	// is to be finalized has to be the last finalized block.

	var finalized uint64
	err := m.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = m.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return fmt.Errorf("could not retrieve final header: %w", err)
	}
	block, err := m.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve pending block: %w", err)
	}
	header := block.Header
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

	// EPOCH: A block inserted into the protocol state is already a valid extension

	epochStatus, err := m.epoch.statuses.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve epoch state: %w", err)
	}
	setup, err := m.epoch.setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		return fmt.Errorf("could not retrieve setup event for current epoch: %w", err)
	}

	payload := block.Payload
	// track protocol events that should be emitted
	var events []func()
	for _, seal := range payload.Seals {
		for _, event := range seal.ServiceEvents {
			switch ev := event.Event.(type) {
			case *flow.EpochSetup:
				events = append(events, func() { m.consumer.EpochSetupPhaseStarted(ev.Counter-1, header) })
			case *flow.EpochCommit:
				events = append(events, func() { m.consumer.EpochCommittedPhaseStarted(ev.Counter-1, header) })
			default:
				return fmt.Errorf("invalid service event type in payload (%T)", event)
			}
		}
	}

	// retrieve the final view of the current epoch w.r.t. the parent block
	finalView, err := m.AtBlockID(header.ParentID).Epochs().Current().FinalView()
	if err != nil {
		return fmt.Errorf("could not get parent epoch final view: %w", err)
	}

	// if this block's view exceeds the final view of its parent's current epoch,
	// this block begins the next epoch
	if header.View > finalView {
		events = append(events, func() { m.consumer.EpochTransition(setup.Counter, header) })
	}

	// FINALLY: any block that is finalized is already a valid extension;
	// in order to make it final, we need to do just three things:
	// 1) Map its height to its index; there can no longer be other blocks at
	// this height, as it becomes immutable.
	// 2) Forward the last finalized height to its height as well. We now have
	// a new last finalized height.
	// 3) Forward the last seled height to the height of the block its last
	// seal sealed. This could actually stay the same if it has no seals in its
	// payload, in which case the parent's seal is the same.

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

	// FOURTH: metrics and events

	m.metrics.FinalizedHeight(header.Height)
	m.metrics.SealedHeight(sealed.Height)
	m.metrics.BlockFinalized(block)

	m.consumer.BlockFinalized(header)
	for _, emit := range events {
		emit()
	}

	for _, seal := range block.Payload.Seals {

		// get each sealed block for sealed metrics
		sealed, err := m.blocks.ByID(seal.BlockID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed block (%x): %w", seal.BlockID, err)
		}

		m.metrics.BlockSealed(sealed)
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
			return nil, fmt.Errorf("missing setup event for starting next epoch")
		}
		if parentStatus.NextEpoch.CommitID == flow.ZeroID {
			return nil, fmt.Errorf("missing commit event for starting next epoch")
		}
		status, err := flow.NewEpochStatus(
			block.ID(),
			parentStatus.NextEpoch.SetupID, parentStatus.NextEpoch.CommitID,
			flow.ZeroID, flow.ZeroID,
		)
		return status, err
	}

	// Block is in the same epoch as its parent, re-use the same epoch status
	// IMPORTANT: copy the status to avoid modifying the parent status in the cache
	status, err := flow.NewEpochStatus(
		parentStatus.FirstBlockID,
		parentStatus.CurrentEpoch.SetupID, parentStatus.CurrentEpoch.CommitID,
		parentStatus.NextEpoch.SetupID, parentStatus.NextEpoch.CommitID,
	)
	return status, err
}

// handleServiceEvents checks the service events within the seals of a block.
// It returns an error if there are any invalid, malformed, or duplicate events,
// in which case this block should be rejected.
//
// If the service events are valid, or there are no service events, it returns
// a slice of Badger operations to apply while storing the block. This includes
// an operation to index the epoch status for every block, and operations to
// insert service events for blocks that include them.
func (m *FollowerState) handleServiceEvents(block *flow.Block) ([]func(*badger.Txn) error, error) {

	// Determine epoch status for block's CURRENT epoch.
	//
	// This yields the tentative protocol state BEFORE applying the block payload.
	// As we don't have slashing yet, there is nothing in the payload which could
	// modify the protocol state for the current epoch.
	epochStatus, err := m.epochStatus(block.Header)
	if err != nil {
		return nil, fmt.Errorf("could not determine epoch status: %w", err)
	}

	activeSetup, err := m.epoch.setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current epoch setup event: %w", err)
	}
	counter := activeSetup.Counter

	// keep track of DB operations to apply when inserting this block
	var ops []func(*badger.Txn) error

	// The payload might contain epoch preparation service events for the next
	// epoch. In this case, we need to update the tentative protocol state.
	// We need to validate whether all information is available in the protocol
	// state to go to the next epoch when needed. In cases where there is a bug
	// in the smart contract, it could be that this happens too late and the
	// chain finalization should halt.
	for _, seal := range block.Payload.Seals {
		for _, event := range seal.ServiceEvents {

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

				// The final view needs to be after the current epoch final view.
				// NOTE: This kind of operates as an overflow check for the other checks.
				if ev.FinalView <= activeSetup.FinalView {
					return nil, state.NewInvalidExtensionErrorf("next epoch must be after current epoch (%d <= %d)", ev.FinalView, activeSetup.FinalView)
				}

				// Finally, the epoch setup event must contain all necessary information.
				err = validSetup(ev)
				if err != nil {
					return nil, state.NewInvalidExtensionErrorf("invalid epoch setup: %s", err)
				}

				// cache the first view to simplify epoch queries later on
				ev.FirstView = activeSetup.FinalView + 1

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
				err = validCommit(ev, setup)
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
