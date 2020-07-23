// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/cadence/encoding/json"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(root *flow.Block, result *flow.ExecutionResult, seal *flow.Seal) error {
	return operation.RetryOnConflict(m.state.db.Update, func(tx *badger.Txn) error {

		// NEW: bootstrapping from an arbitrary states requires an execution result and block seal
		// as input; we need to verify them against each other and against the root block

		if result.BlockID != root.ID() {
			return fmt.Errorf("root execution result for wrong block (%x != %x)", result.BlockID, root.ID())
		}

		if seal.BlockID != root.ID() {
			return fmt.Errorf("root block seal for wrong block (%x != %x)", seal.BlockID, root.ID())
		}

		if seal.ResultID != result.ID() {
			return fmt.Errorf("root block seal for wrong execution result (%x != %x)", seal.ResultID, result.ID())
		}

		// EPOCHS: If we bootstrap with epochs, we no longer need identities as a payload to the root block; instead, we
		// want to see two system events with all necessary information: one epoch setup and one epoch commit.

		// check that we have one epoch setup and one epoch commit event
		if len(seal.ServiceEvents) != 2 {
			return fmt.Errorf("root block seal must contain two system events (have %d)", len(seal.ServiceEvents))
		}
		event1 := seal.ServiceEvents[0]
		event2 := seal.ServiceEvents[1]
		if event1.Type != flow.EventEpochSetup {
			return fmt.Errorf("first system event is not epoch setup (%s)", event1.Type)
		}
		if event2.Type != flow.EventEpochCommit {
			return fmt.Errorf("second system event is not epoch commit (%s)", event2.Type)
		}

		// decode the event payloads into cadence values
		value1, err := json.Decode(event1.Payload)
		if err != nil {
			return fmt.Errorf("could not decode first system event: %w", err)
		}
		value2, err := json.Decode(event2.Payload)
		if err != nil {
			return fmt.Errorf("could not decode second system event: %w", err)
		}

		// use type assertion to get the native types
		// NOTE: this should always work, as we checked the types
		// earlier, and will panic otherwise anyway
		setup := value1.ToGoValue().(*flow.EpochSetup)
		commit := value2.ToGoValue().(*flow.EpochCommit)

		// make sure they both refer to the same epoch
		if setup.Counter != commit.Counter {
			return fmt.Errorf("epoch setup counter differs from epoch commit counter (%d != %d)", setup.Counter, commit.Counter)
		}

		// FIRST: validate the root block and its payload

		// NOTE: we might need to relax these restrictions and find a way to process the
		// payload of the root block once we implement epochs

		// the root block should have an empty guarantee payload
		if len(root.Payload.Guarantees) > 0 {
			return fmt.Errorf("root block must not have guarantees")
		}

		// the root block should have an empty seal payload
		if len(root.Payload.Seals) > 0 {
			return fmt.Errorf("root block must not have seals")
		}

		// the root identities need at least one identity for each role
		roles := make(map[flow.Role]uint)
		for _, identity := range setup.Identities {
			roles[identity.Role]++
		}
		if roles[flow.RoleConsensus] < 1 {
			return fmt.Errorf("need at least one consensus node")
		}
		if roles[flow.RoleCollection] < 1 {
			return fmt.Errorf("need at least one collection node")
		}
		if roles[flow.RoleExecution] < 1 {
			return fmt.Errorf("need at least one execution node")
		}
		if roles[flow.RoleVerification] < 1 {
			return fmt.Errorf("need at least one verification node")
		}

		// we should have at least one root collection cluster
		if setup.Clusters.Size() == 0 {
			return fmt.Errorf("need at least one collection cluster")
		}

		// the root identities should not contain duplicates
		identLookup := make(map[flow.Identifier]struct{})
		for _, identity := range setup.Identities {
			_, ok := identLookup[identity.NodeID]
			if ok {
				return fmt.Errorf("duplicate node identifier (%x)", identity.NodeID)
			}
			identLookup[identity.NodeID] = struct{}{}
		}

		// the root identities should not contain duplicate addresses
		addrLookup := make(map[string]struct{})
		for _, identity := range setup.Identities {
			_, ok := addrLookup[identity.Address]
			if ok {
				return fmt.Errorf("duplicate node address (%x)", identity.Address)
			}
			addrLookup[identity.Address] = struct{}{}
		}

		// the root identities should all have a non-zero stake
		for _, identity := range setup.Identities {
			if identity.Stake == 0 {
				return fmt.Errorf("zero stake identity (%x)", identity.NodeID)
			}
		}

		// TODO: Determine what other compliance checks we want to execute on
		// the epoch events of the root seal.

		// SECOND: insert the initial protocol state data into the database

		// 1) insert the root block with its payload into the state and index it
		err = m.state.blocks.Store(root)
		if err != nil {
			return fmt.Errorf("could not insert root block: %w", err)
		}
		err = operation.IndexBlockHeight(root.Header.Height, root.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root block: %w", err)
		}
		err = operation.InsertBlockChildren(root.ID(), nil)(tx)
		if err != nil {
			return fmt.Errorf("could not initialize root child index: %w", err)
		}

		// 2) insert the root execution result into the database and index it
		err = operation.InsertExecutionResult(result)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root result: %w", err)
		}
		err = operation.IndexExecutionResult(root.ID(), result.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root result: %w", err)
		}

		// 3) insert the root block seal into the database and index it
		err = operation.InsertSeal(seal.ID(), seal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root seal: %w", err)
		}
		err = operation.IndexBlockSeal(root.ID(), seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index root block seal: %w", err)
		}

		// 4) initialize the current protocol state values
		err = operation.InsertStartedView(root.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertVotedView(root.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertRootHeight(root.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert root height: %w", err)
		}
		err = operation.InsertFinalizedHeight(root.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
		err = operation.InsertSealedHeight(root.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed height: %w", err)
		}

		// 5) initialize values related to the epoch logic
		err = operation.InsertEpochCounter(setup.Counter)(tx)
		if err != nil {
			return fmt.Errorf("could not insert epoch counter: %w", err)
		}
		err = operation.InsertEpochSetup(setup.Counter, setup)(tx)
		if err != nil {
			return fmt.Errorf("could not insert epoch seed: %w", err)
		}
		err = operation.InsertEpochCommit(commit.Counter, commit)(tx)
		if err != nil {
			return fmt.Errorf("could not insert eoch end: %w", err)
		}

		m.state.metrics.FinalizedHeight(root.Header.Height)
		m.state.metrics.BlockFinalized(root)

		m.state.metrics.SealedHeight(root.Header.Height)
		m.state.metrics.BlockSealed(root)

		return nil
	})
}

func (m *Mutator) Extend(candidate *flow.Block) error {

	// FIRST: We do some initial cheap sanity checks. Currently, only the
	// root block can contain identities. We also want to make sure that the
	// payload hash has been set correctly.

	header := candidate.Header
	payload := candidate.Payload
	if payload.Hash() != header.PayloadHash {
		return state.NewInvalidExtensionError("payload integrity check failed")
	}

	// SECOND: Next, we can check whether the block is a valid descendant of the
	// parent. It should have the same chain ID and a height that is one bigger.

	parent, err := m.state.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent: %w", err)
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

	var finalized uint64
	err = m.state.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = m.state.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return fmt.Errorf("could not lookup finalized block: %w", err)
	}

	ancestorID := header.ParentID
	for ancestorID != finalID {
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor (%x): %w", ancestorID, err)
		}
		if ancestor.Height < finalized {
			return state.NewOutdatedExtensionErrorf("candidate block conflicts with finalized state (ancestor: %d final: %d)",
				ancestor.Height, finalized)
		}
		ancestorID = ancestor.ParentID
	}

	// FOURTH: The header is now fully validated. Next is the guarantee part of
	// the payload compliance check. None of the blocks should have included a
	// guarantee that was expired at the block height, nor should it have been
	// included in any previous payload.

	// we only look as far back for duplicates as the transaction expiry limit;
	// if a guarantee was included before that, we will disqualify it on the
	// basis of the reference block anyway
	limit := header.Height - uint64(m.state.expiry)
	if limit > header.Height { // overflow check
		limit = 0
	}

	// look up the root height so we don't look too far back
	// initially this is the genesis block height (aka 0).
	var rootHeight uint64
	err = m.state.db.View(operation.RetrieveRootHeight(&rootHeight))
	if err != nil {
		return fmt.Errorf("could not retrieve root block height: %w", err)
	}
	if limit < rootHeight {
		limit = rootHeight
	}

	// build a list of all previously used guarantees on this part of the chain
	ancestorID = header.ParentID
	lookup := make(map[flow.Identifier]struct{})
	for {
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor header (%x): %w", ancestorID, err)
		}
		index, err := m.state.index.ByBlockID(ancestorID)
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
		ref, err := m.state.headers.ByBlockID(guarantee.ReferenceBlockID)
		if err != nil {
			return fmt.Errorf("could not get reference block (%x): %w", guarantee.ReferenceBlockID, err)
		}

		// if the guarantee references a block with expired height, error
		if ref.Height < limit {
			return state.NewInvalidExtensionErrorf("payload includes expired guarantee (height: %d, limit: %d)",
				ref.Height, limit)
		}
	}

	// FIFTH: For compliance of the seal payload, we need them to create a valid
	// chain of seals on our branch of the chain, starting at the block directly
	// after the last sealed block all the way to at most the parent. We use
	// deterministic lookup by height for the finalized part of the chain and
	// then make a list of unfinalized blocks for the remainder, if any seals
	// remain.

	// map each seal to the block it is sealing for easy lookup; we will need to
	// successfully connect _all_ of these seals to the last sealed block for
	// the payload to be valid
	byBlock := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range payload.Seals {
		byBlock[seal.BlockID] = seal
	}
	if len(payload.Seals) > len(byBlock) {
		return state.NewInvalidExtensionErrorf("multiple seals for the same block")
	}

	// get the parent's block seal, which constitutes the beginning of the
	// sealing chain; if no seals are part of the payload, it will also be used
	// for the candidate block, which remains at the same sealed state
	last, err := m.state.seals.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent seal (%x): %w", header.ParentID, err)
	}

	// get the last sealed block; we use its height to iterate forwards through
	// the finalized blocks which still need sealing
	sealed, err := m.state.headers.ByBlockID(last.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed block (%x): %w", last.BlockID, err)
	}

	// we now go from last sealed height plus one to finalized height and check
	// if we have the seal for each of them step by step; often we will not even
	// enter this loop, because last sealed height is higher than finalized
	for height := sealed.Height + 1; height <= finalized; height++ {
		if len(byBlock) == 0 {
			break
		}
		header, err := m.state.headers.ByHeight(height)
		if err != nil {
			return fmt.Errorf("could not get block for sealed height (%d): %w", height, err)
		}
		blockID := header.ID()
		next, found := byBlock[blockID]
		if !found {
			return state.NewInvalidExtensionErrorf("chain of seals broken for finalized (missing: %x)", blockID)
		}
		if !bytes.Equal(next.InitialState, last.FinalState) {
			return state.NewInvalidExtensionError("seal execution states do not connect in finalized")
		}
		delete(byBlock, blockID)
		last = next
	}

	// NOTE: We could skip the remaining part in case no seals are left; it is,
	// however, cheap, and it's what we will always do during normal operation,
	// where we only seal the last 1-3 blocks, which are not yet finalized.

	// Once we have filled in seals for all finalized blocks we need to check
	// the non-finalized blocks backwards; collect all of them, from direct
	// parent to just before finalized, and see if we can use up the rest of the
	// seals. We need to stop collecting ancestors either when reaching the
	// finalized state, or when reaching the last sealed block.
	ancestorID = header.ParentID
	var pendingIDs []flow.Identifier
	for ancestorID != finalID && ancestorID != last.BlockID {
		pendingIDs = append(pendingIDs, ancestorID)
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not get sealable ancestor (%x): %w", ancestorID, err)
		}
		ancestorID = ancestor.ParentID
	}
	for i := len(pendingIDs) - 1; i >= 0; i-- {
		if len(byBlock) == 0 {
			break
		}
		pendingID := pendingIDs[i]
		next, found := byBlock[pendingID]
		if !found {
			return state.NewInvalidExtensionErrorf("chain of seals broken for pending (missing: %x)", pendingID)
		}
		if !bytes.Equal(next.InitialState, last.FinalState) {
			return state.NewInvalidExtensionErrorf("seal execution states do not connect in pending")
		}
		delete(byBlock, pendingID)
		last = next
	}

	// this is just a sanity check; at this point, no seals should be left
	if len(byBlock) > 0 {
		return fmt.Errorf("not all seals connected to state (left: %d)", len(byBlock))
	}

	// SIXTH: In case any of the payload seals includes system events, we need to
	// check if they are valid and must apply them to the protocol state as needed.

	// retrieve the current epoch counter and collect system events from seals
	var counter uint64
	err = m.state.db.View(operation.RetrieveEpochCounter(&counter))
	if err != nil {
		return fmt.Errorf("could not retrieve epoch counter: %w", err)
	}
	var events []*flow.Event
	for _, seal := range payload.Seals {
		events = append(events, seal.ServiceEvents...)
	}

	// NOTE: We could check that we have at most two here, but using more
	// sophisticated checks that catch invalid events one by one makes
	// the code more extensible in the future.

	// Let's first establish the status quo of the current epoch. This function
	// checks if we already had an epoch setup or commit event in the history of
	// the blockchain, both the finalized and the pending part.
	didSetup, didCommit, err := m.epochStatus(counter+1, header.ParentID)
	if err != nil {
		return fmt.Errorf("could not check epoch status: %w", err)
	}

	// for each event, check if it is valid
	for _, event := range events {

		// decode the event first; should work for all
		value, err := json.Decode(event.Payload)
		if err != nil {
			return fmt.Errorf("could not decode service event: %w", err)
		}

		// check if the event is an epoch setup event
		if event.Type == flow.EventEpochSetup {

			// we should only have a single epoch setup event per epoch
			if didSetup {
				return fmt.Errorf("duplicate epoch setup service event")
			}

			// type assert the event and check if the counter is valid
			setup := value.ToGoValue().(*flow.EpochCommit)
			if setup.Counter != counter+1 {
				return fmt.Errorf("invalid epoch setup event counter (%d => %d)", counter, setup.Counter)
			}

			// TODO: Determine what other compliance checks we want to run on the epoch setup event.
			// => https://github.com/dapperlabs/flow-go/issues/4437

			// make sure we don't allow multiple setup events per payload
			didSetup = true

			continue
		}

		// check if the event is an epoch commit event
		if event.Type == flow.EventEpochCommit {

			// we should only have a single epoch commit event per epoch
			if didCommit {
				return fmt.Errorf("duplicate epoch commit service event")
			}

			// an epoch commit event is only valid if it was preceeded by a setup event
			if !didSetup {
				return fmt.Errorf("missing epoch setup for epoch commit")
			}

			// type assert the event and check if the counter is valid
			commit := value.ToGoValue().(*flow.EpochCommit)
			if commit.Counter != counter+1 {
				return fmt.Errorf("invalid epoch commit event counter (%d => %d)", counter, commit.Counter)
			}

			// TODO: Determine what other compliance checks we want to run on the epoch commit event.
			// => https://github.com/dapperlabs/flow-go/issues/4437

			// make sure we don't allow multiple commit events per payload
			didCommit = true

			continue
		}

		return fmt.Errorf("invalid service event type in block seal (%s)", event.Type)
	}

	// FINALLY: Both the header itself and its payload are in compliance with the
	// protocol state. We can now store the candidate block, as well as adding
	// its final seal to the seal index and initializing its children index.

	err = m.state.blocks.Store(candidate)
	if err != nil {
		return fmt.Errorf("could not store candidate block: %w", err)
	}
	blockID := candidate.ID()
	err = operation.RetryOnConflict(m.state.db.Update, func(tx *badger.Txn) error {
		err := operation.IndexBlockSeal(blockID, last.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index candidate seal: %w", err)
		}
		// add an empty child index for the added block
		err = operation.InsertBlockChildren(blockID, nil)(tx)
		if err != nil {
			return fmt.Errorf("could not initialize children index: %w", err)
		}
		// index the added block as a child of its parent
		err = procedure.IndexBlockChild(candidate.Header.ParentID, blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not add to parent index: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not execute state extension: %w", err)
	}

	return nil
}

func (m *Mutator) Finalize(blockID flow.Identifier) error {

	// FIRST: The finalize call on the protocol state can only finalize one
	// block at a time. This implies that the parent of the pending block that
	// is to be finalized has to be the last finalized block.

	var finalized uint64
	err := m.state.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = m.state.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return fmt.Errorf("could not retrieve final header: %w", err)
	}
	block, err := m.state.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve pending block: %w", err)
	}
	header := block.Header
	if header.ParentID != finalID {
		return fmt.Errorf("can only finalize child of last finalized block")
	}

	// SECOND: We also want to update the last sealed height. Retrieve the block
	// seal indexed for the block and retrieve the block that was sealed by it.

	last, err := m.state.seals.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not look up sealed header: %w", err)
	}
	sealed, err := m.state.headers.ByBlockID(last.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed header: %w", err)
	}

	// THIRD: If we have system events in any of the seals, we should insert
	// them into the protocol state accordingly on finalization. We create a
	// list of operations to be applied on top of the rest.

	payload := block.Payload
	var ops []func(*badger.Txn) error
	for _, seal := range payload.Seals {
		for _, event := range seal.ServiceEvents {
			value, err := json.Decode(event.Payload)
			if err != nil {
				return fmt.Errorf("could not decode system event: %w", err)
			}
			switch event.Type {
			case flow.EventEpochSetup:
				setup := value.ToGoValue().(*flow.EpochSetup)
				ops = append(ops, operation.InsertEpochSetup(setup.Counter, setup))
			case flow.EventEpochCommit:
				commit := value.ToGoValue().(*flow.EpochCommit)
				ops = append(ops, operation.InsertEpochCommit(commit.Counter, commit))
			default:
				return fmt.Errorf("invalid system event type in payload (%s)", event.Type)
			}
		}
	}

	// FINALLY: A block inserted into the protocol state is already a valid
	// extension; in order to make it final, we need to do just three things:
	// 1) Map its height to its index; there can no longer be other blocks at
	// this height, as it becomes immutable.
	// 2) Forward the last finalized height to its height as well. We now have
	// a new last finalized height.
	// 3) Forward the last seled height to the height of the block its last
	// seal sealed. This could actually stay the same if it has no seals in its
	// payload, in which case the parent's seal is the same.

	err = operation.RetryOnConflict(m.state.db.Update, func(tx *badger.Txn) error {
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
		for _, op := range ops {
			err = op(tx)
			if err != nil {
				return fmt.Errorf("could not apply additional operation: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not execute finalization: %w", err)
	}

	// FOURTH: metrics

	m.state.metrics.FinalizedHeight(header.Height)
	m.state.metrics.SealedHeight(sealed.Height)

	m.state.metrics.BlockFinalized(block)

	for _, seal := range block.Payload.Seals {

		// get each sealed block for sealed metrics
		sealed, err := m.state.blocks.ByID(seal.BlockID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed block (%x): %w", seal.BlockID, err)
		}

		m.state.metrics.BlockSealed(sealed)
	}

	return nil
}

func (m *Mutator) epochStatus(counter uint64, ancestorID flow.Identifier) (bool, bool, error) {

	// First, we check if both epoch events have already been finalized; if they have, we don't
	// need to check anything else.
	setupFinalized, err := m.setupFinalized(counter)
	if err != nil {
		return false, false, fmt.Errorf("could not check epoch setup finalization: %w", err)
	}
	commitFinalized, err := m.commitFinalized(counter)
	if err != nil {
		return false, false, fmt.Errorf("could not check next epoch commit finalization: %w", err)
	}
	if setupFinalized && commitFinalized {
		return true, true, nil
	}

	// This code is only to prepare the below loop. We could inject them as parameters, but it
	// is a little less elegant, and the values should be in the badger cache anyway. This keeps
	// the function signature a bit simpler.
	var finalized uint64
	err = m.state.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return false, false, fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = m.state.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return false, false, fmt.Errorf("could not lookup finalized block: %w", err)
	}

	// Next, we want to check if we find the events in one of the pending blocks. We shouldn't
	// have to check the order of events here; if they were committed in the wrong order, we
	// already have an invalid protocol state that could be finalized and everything is broken.
	// This allows us to ignore the finalized status of the setup event.
	setupPending := false
	commitPending := false
	for ancestorID != finalID {

		// we need to get the ancestor to check its height for validity; it could be that the
		// block is connected to an obsolete branch of the blockchain that can no longer be
		// finalized, in which case we will never reach the finalID
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return false, false, fmt.Errorf("could not retrieve ancestor (%x): %w", ancestorID, err)
		}
		if ancestor.Height < finalized {
			return false, false, state.NewOutdatedExtensionErrorf("candidate block conflicts with finalized state (ancestor: %d final: %d)", ancestor.Height, finalized)
		}

		// next, we retrieve the payload to look at the service events for all block seals included
		// on the pending part of the blockchain; if both events were found, it means we can stop
		// looking as we should only have one of each for the compliant part of the chain
		payload, err := m.state.payloads.ByBlockID(ancestorID)
		if err != nil {
			return false, false, fmt.Errorf("could not retrieve payload (%x): %w", ancestorID, err)
		}
		for _, seal := range payload.Seals {
			for _, event := range seal.ServiceEvents {
				if setupPending && commitPending {
					break
				}
				if event.Type == flow.EventEpochSetup {
					setupPending = true
					continue
				}
				if event.Type == flow.EventEpochCommit {
					commitPending = true
					continue
				}
			}
		}
		ancestorID = ancestor.ParentID
	}

	return setupPending, commitPending, nil
}

func (m *Mutator) setupFinalized(counter uint64) (bool, error) {
	err := m.state.db.View(operation.RetrieveEpochSetup(counter, &flow.EpochSetup{}))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (m *Mutator) commitFinalized(counter uint64) (bool, error) {
	err := m.state.db.View(operation.RetrieveEpochCommit(counter, &flow.EpochCommit{}))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return false, err
}
