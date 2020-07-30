// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/epoch"
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

		// We should have exactly two service events, one epoch setup and one epoch commit.
		if len(seal.ServiceEvents) != 2 {
			return fmt.Errorf("root block seal must contain two system events (have %d)", len(seal.ServiceEvents))
		}
		var services []interface{}
		for _, event := range seal.ServiceEvents {
			service, err := epoch.ServiceEvent(event)
			if err != nil {
				return fmt.Errorf("could not decode service event: %w", err)
			}
			services = append(services, service)
		}
		setup, valid := services[0].(*epoch.Setup)
		if !valid {
			return fmt.Errorf("first service event should be epoch setup (%T)", services[0])
		}
		commit, valid := services[1].(*epoch.Commit)
		if !valid {
			return fmt.Errorf("second event should be epoch commit (%T)", services[1])
		}

		// They should both have the same epoch counter to be valid.
		if setup.Counter != commit.Counter {
			return fmt.Errorf("epoch setup counter differs from epoch commit counter (%d != %d)", setup.Counter, commit.Counter)
		}

		// They should also both be valid within themselves.
		err := validSetup(setup)
		if err != nil {
			return fmt.Errorf("invalid epoch setup event: %w", err)
		}
		err = validCommit(setup, commit)
		if err != nil {
			return fmt.Errorf("invalid epoch commit event: %w", err)
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
		err = operation.IndexEpochStart(setup.Counter, root.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not index epoch start: %w", err)
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
	limit := header.Height - m.state.cfg.transactionExpiry
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

	// This is just a sanity check; at this point, no seals should be left.
	if len(byBlock) > 0 {
		return fmt.Errorf("not all seals connected to state (left: %d)", len(byBlock))
	}

	// SIXTH: In case any of the payload seals includes system events, we need to
	// check if they are valid and must apply them to the protocol state as needed.

	// Retrieve the current epoch counter and the current epoch's service events.
	var counter uint64
	err = m.state.db.View(operation.RetrieveEpochCounter(&counter))
	if err != nil {
		return fmt.Errorf("could not retrieve epoch counter: %w", err)
	}
	var activeSetup epoch.Setup
	err = m.state.db.View(operation.RetrieveEpochSetup(counter, &activeSetup))
	if err != nil {
		return fmt.Errorf("could not retrieve current epoch setup: %w", err)
	}

	// Let's first establish the status quo of the current epoch. This function
	// checks if we already had an epoch setup or commit event in the history of
	// the blockchain, both the finalized and the pending part.
	didSetup, didCommit, err := m.epochStatus(counter+1, header.ParentID)
	if err != nil {
		return fmt.Errorf("could not check epoch status: %w", err)
	}

	// For each service event included in the payload, check whether it
	// is compliant with the protocol rules.
	// NOTE: We could check that we have at most two service eventshere,
	// but using more granular checks that catch invalid events one by one
	// makes the code more extensible in the future.
	for _, seal := range payload.Seals {
		for _, event := range seal.ServiceEvents {

			// Convert the event into a strongly typed service event.
			// TODO: We might want to do this upon creation of the seals
			// instead; the only question is how to encode it over the
			// network nicely, as a slice of interfaces needs additional
			// meta information, and having two nil fields for most seals
			// is ugly.
			service, err := epoch.ServiceEvent(event)
			if err != nil {
				return fmt.Errorf("could not decode service event: %w", err)
			}

			switch ev := service.(type) {

			case *epoch.Setup:

				// We should only have a single epoch setup event per epoch.
				if didSetup {
					return fmt.Errorf("duplicate epoch setup service event")
				}

				// The setup event should have the counter increased by one.
				if ev.Counter != counter+1 {
					return fmt.Errorf("next epoch setup has invalid counter (%d => %d)", counter, ev.Counter)
				}

				// The final view needs to be after the current epoch final view.
				// NOTE: This kind of operates as an overflow check for the other checks.
				if ev.FinalView <= activeSetup.FinalView {
					return fmt.Errorf("next epoch must be after current epoch (%d <= %d)", ev.FinalView, activeSetup.FinalView)
				}

				// Finally, the epoch setup event must contain all necessary information.
				err = validSetup(ev)
				if err != nil {
					return fmt.Errorf("invalid epoch setup: %w", err)
				}

				// Make sure to disallow multiple commit events per payload.
				didSetup = true

			case *epoch.Commit:

				// We should only have a single epoch commit event per epoch.
				if didCommit {
					return fmt.Errorf("duplicate epoch commit service event")
				}

				// The epoch setup event needs to happen before the commit.
				if !didSetup {
					return fmt.Errorf("missing epoch setup for epoch commit")
				}

				// The commit event should have the counter increased by one.
				if ev.Counter != counter+1 {
					return fmt.Errorf("next epoch commit has invalid counter (%d => %d)", counter, ev.Counter)
				}

				// Finally, the commit should commit all the necessary information.
				var setup epoch.Setup
				err = m.state.db.View(operation.RetrieveEpochSetup(ev.Counter, &setup))
				if err != nil {
					return fmt.Errorf("could not retrieve next epoch setup: %w", err)
				}
				err = validCommit(&setup, ev)
				if err != nil {
					return fmt.Errorf("invalid epoch commit: %w", err)
				}

				// Make sure to disallow multiple commit events per payload.
				didCommit = true

			default:
				// NOTE: This is already handled as error in the decoding; if
				// we decode an event successfully, it should be processed, so
				// all we need to do here is do a warning log that there are
				// unprocessed service events.
			}
		}
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
			service, err := epoch.ServiceEvent(event)
			if err != nil {
				return fmt.Errorf("could not decode service event: %w", err)
			}
			switch ev := service.(type) {
			case *epoch.Setup:
				ops = append(ops, operation.InsertEpochSetup(ev.Counter, ev))
			case *epoch.Commit:
				ops = append(ops, operation.InsertEpochCommit(ev.Counter, ev))
			default:
				return fmt.Errorf("invalid service event type in payload (%s)", event.Type)
			}
		}
	}

	// EPOCH: We need to validate whether all information is available in the
	// protocol state to go to the next epoch when needed. In cases where there
	// is a bug in the smart contract, it could be that this happens too late
	// and the chain finalization should halt.
	// We also map the epoch to the height of its last finalized block; this is
	// important in order to efficiently be able to look up epoch snapshots.

	var counter uint64
	err = m.state.db.View(operation.RetrieveEpochCounter(&counter))
	if err != nil {
		return fmt.Errorf("could not retrieve epoch counter: %w", err)
	}
	var setup epoch.Setup
	err = m.state.db.View(operation.RetrieveEpochSetup(counter, &setup))
	if err != nil {
		return fmt.Errorf("could not retrieve epoch setup: %w", err)
	}
	if header.View > setup.FinalView {
		didSetup, didCommit, err := m.epochStatus(counter+1, finalID)
		if err != nil {
			return fmt.Errorf("could not check epoch status: %w", err)
		}
		if !didSetup || !didCommit {
			return fmt.Errorf("missing epoch transition event(s)!")
		}
		counter = counter + 1
		ops = append(ops, operation.UpdateEpochCounter(counter))
		ops = append(ops, operation.InsertEpochHeight(counter, header.Height))
	} else {
		ops = append(ops, operation.UpdateEpochHeight(counter, header.Height))
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
	err := m.state.db.View(operation.RetrieveEpochSetup(counter, &epoch.Setup{}))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (m *Mutator) commitFinalized(counter uint64) (bool, error) {
	err := m.state.db.View(operation.RetrieveEpochCommit(counter, &epoch.Commit{}))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return false, err
}
