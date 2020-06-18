// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
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

		// FIRST: validate the root block and its payload

		// the root block can not have a parent
		if root.Header.ParentID != flow.ZeroID {
			return errors.New("root block must not have parent")
		}

		// the root block should have an empty guarantee payload
		if len(root.Payload.Guarantees) > 0 {
			return fmt.Errorf("root block must not have guarantees")
		}

		// the root block should have an empty seal payload
		if len(root.Payload.Seals) > 0 {
			return fmt.Errorf("root block must not have seals")
		}

		// the root block needs at least one identity for each role
		roles := make(map[flow.Role]uint)
		for _, identity := range root.Payload.Identities {
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

		// the root block should not contain duplicate identities
		identLookup := make(map[flow.Identifier]struct{})
		for _, identity := range root.Payload.Identities {
			_, ok := identLookup[identity.NodeID]
			if ok {
				return fmt.Errorf("duplicate node identifier (%x)", identity.NodeID)
			}
			identLookup[identity.NodeID] = struct{}{}
		}

		// the root block identities should not contain duplicate addresses
		addrLookup := make(map[string]struct{})
		for _, identity := range root.Payload.Identities {
			_, ok := addrLookup[identity.Address]
			if ok {
				return fmt.Errorf("duplicate node address (%x)", identity.Address)
			}
			addrLookup[identity.Address] = struct{}{}
		}

		// the root block identities should all have a non-zero stake
		for _, identity := range root.Payload.Identities {
			if identity.Stake == 0 {
				return fmt.Errorf("zero stake identity (%x)", identity.NodeID)
			}
		}

		// SECOND: insert the initial protocol state data into the database

		// 1) insert the root block with its payload into the state and index it
		err := m.state.blocks.Store(root)
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
	if len(payload.Identities) > 0 {
		return fmt.Errorf("extend block has identities")
	}
	if payload.Hash() != header.PayloadHash {
		return fmt.Errorf("payload integrity check failed")
	}

	// SECOND: Next, we can check whether the block is a valid descendant of the
	// parent. It should have the same chain ID and a height that is one bigger.

	parent, err := m.state.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve parent: %w", err)
	}
	if header.ChainID != parent.ChainID {
		return fmt.Errorf("candidate built for invalid chain (candidate: %s, parent: %s)", header.ChainID, parent.ChainID)
	}
	if header.Height != parent.Height+1 {
		return fmt.Errorf("candidate built with invalid height (candidate: %d, parent: %d)", header.Height, parent.Height)
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
			return fmt.Errorf("candidate block conflicts with finalized state (ancestor: %d final: %d)", ancestor.Height, finalized)
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
			return fmt.Errorf("could not retrieve ancestor payload (%x): %w", ancestorID, err)
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
			return fmt.Errorf("payload includes duplicate guarantee (%x)", guarantee.ID())
		}

		// get the reference block to check expiry
		ref, err := m.state.headers.ByBlockID(guarantee.ReferenceBlockID)
		if err != nil {
			return fmt.Errorf("could not get reference block (%x): %w", guarantee.ReferenceBlockID, err)
		}

		// if the guarantee references a block with expired height, error
		if ref.Height < limit {
			return fmt.Errorf("payload includes expired guarantee (height: %d, limit: %d)", ref.Height, limit)
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
		return fmt.Errorf("multiple seals for the same block")
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
			return fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blockID := header.ID()
		next, found := byBlock[blockID]
		if !found {
			return fmt.Errorf("chain of seals broken for finalized (missing: %x)", blockID)
		}
		if !bytes.Equal(next.InitialState, last.FinalState) {
			return fmt.Errorf("seal execution states do not connect in finalized")
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
			return fmt.Errorf("chain of seals broken for pending (missing: %x)", pendingID)
		}
		if !bytes.Equal(next.InitialState, last.FinalState) {
			return fmt.Errorf("seal execution states do not connect in pending")
		}
		delete(byBlock, pendingID)
		last = next
	}

	// this is just a sanity check; at this point, no seals should be left
	if len(byBlock) > 0 {
		return fmt.Errorf("not all seals connected to state (left: %d)", len(byBlock))
	}

	// SIXTH: Both the header itself and its payload are in compliance with the
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
		err = operation.InsertBlockChildren(blockID, nil)(tx)
		if err != nil {
			return fmt.Errorf("could not initialize children index: %w", err)
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
	header, err := m.state.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve pending header: %w", err)
	}
	if header.ParentID != finalID {
		return fmt.Errorf("can only finalized child of last finalized block")
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

	// THIRD: A block inserted into the protocol state is already a valid
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
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not execute finalization: %w", err)
	}

	// FOURTH: metrics

	m.state.metrics.FinalizedHeight(header.Height)
	m.state.metrics.SealedHeight(sealed.Height)

	// get the finalized block for finalized metrics
	final, err := m.state.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve finalized block: %w", err)
	}

	m.state.metrics.BlockFinalized(final)

	for _, seal := range final.Payload.Seals {

		// get each sealed block for sealed metrics
		sealed, err := m.state.blocks.ByID(seal.BlockID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed block (%x): %w", seal.BlockID, err)
		}

		m.state.metrics.BlockSealed(sealed)
	}

	return nil
}
