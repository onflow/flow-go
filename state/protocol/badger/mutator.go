// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(commit flow.StateCommitment, genesis *flow.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// the initial height needs to be height zero
		if genesis.Header.Height != 0 {
			return fmt.Errorf("genesis height must be zero")
		}

		// the parent must be zero hash
		if genesis.Header.ParentID != flow.ZeroID {
			return errors.New("genesis parent must have zero ID")
		}

		// we should have no guarantees
		if len(genesis.Payload.Guarantees) > 0 {
			return fmt.Errorf("genesis block must have zero guarantees")
		}

		// we should have no seals
		if len(genesis.Payload.Seals) > 0 {
			return fmt.Errorf("genesis block must have zero seals")
		}

		// we should have one role of each type at least
		roles := make(map[flow.Role]uint)
		for _, identity := range genesis.Payload.Identities {
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

		// check that we don't have duplicate identity entries
		identLookup := make(map[flow.Identifier]struct{})
		for _, identity := range genesis.Payload.Identities {
			_, ok := identLookup[identity.NodeID]
			if ok {
				return fmt.Errorf("duplicate node identifier (%x)", identity.NodeID)
			}
			identLookup[identity.NodeID] = struct{}{}
		}

		// check identities do not have duplicate addresses
		addrLookup := make(map[string]struct{})
		for _, identity := range genesis.Payload.Identities {
			_, ok := addrLookup[identity.Address]
			if ok {
				return fmt.Errorf("duplicate node address (%x)", identity.Address)
			}
			addrLookup[identity.Address] = struct{}{}
		}

		// for each identity, check it has a non-zero stake
		for _, identity := range genesis.Payload.Identities {
			if identity.Stake == 0 {
				return fmt.Errorf("zero stake identity (%x)", identity.NodeID)
			}
		}

		// insert an empty children lookup for the block
		err := operation.InsertBlockChildren(genesis.ID(), nil)(tx)
		if err != nil {
			return fmt.Errorf("could not insert empty block children: %w", err)
		}

		return nil
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {

	// FIRST: Check that the header is a valid extension of the state; it should
	// connect to the last finalized block.

	candidate, err := m.state.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve candidate header: %w", err)
	}
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

	// In order to check if the candidate connects to the last finalized block
	// 1) Get the parent of the block being checked (candidate first).
	// 2) Check that the parent has a height one smaller than block (only relevant for candidate).
	// 3) Check that the parent is not below the last finalized block.
	// We will either run into one of the error conditions or break the loop when we
	// managed to trace back all the way to the last finalized block.
	height := candidate.Height
	ancestorID := candidate.ParentID
	for ancestorID != finalID {
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not get block's ancestor from state (%x): %w", ancestorID, err)
		}
		if height != ancestor.Height+1 {
			return fmt.Errorf("block needs height equal to ancestor height+1 (%d != %d+1)", height, ancestor.Height)
		}
		if ancestor.Height < finalized {
			return fmt.Errorf("block doesn't connect to finalized state (%d < %d), ancestorID (%v)", ancestor.Height, finalized, ancestorID)
		}
		ancestorID = ancestor.ParentID
		height = ancestor.Height
	}

	// SECOND: Check that the payload has no identities; this is only allowed
	// for the genesis block for now. We also do a sanity check on the payload
	// hash, just to be sure.

	payload, err := m.state.payloads.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve payload: %w", err)
	}
	if len(payload.Identities) > 0 {
		return fmt.Errorf("extend block has identities")
	}
	if payload.Hash() != candidate.PayloadHash {
		return fmt.Errorf("payload integrity check failed")
	}

	// THIRD: Check that all guarantees and all seals in the payload have not
	// yet been included in this branch of the block chain.

	// NOTE: We currently limit ourselves to going back at most 1000 for this
	// check, as this is approximately what we have cached.
	height = candidate.Height - 1
	limit := height - 1000
	if limit > height { // overflow check
		height = 0
	}

	// In order to check if a payload in one of the ancestors already contained
	// any of the seals or the guarantees, we proceed as follows:
	// 1) Build a lookup table for candidate seals & guarantees.
	// 2) Retrieve the header to go to next (should be cached).
	// 3) Retrieve the next ancestor payload (should be cached).
	// 4) Collect the IDs for any duplicates.
	gLookup := make(map[flow.Identifier]struct{})
	for _, guarantee := range payload.Guarantees {
		gLookup[guarantee.ID()] = struct{}{}
	}
	sLookup := make(map[flow.Identifier]struct{})
	for _, seal := range payload.Seals {
		sLookup[seal.ID()] = struct{}{}
	}
	var duplicateGuarIDs flow.IdentifierList
	var duplicateSealIDs flow.IdentifierList
	ancestorID = candidate.ParentID
	for {
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if ancestor.Height <= limit {
			break
		}
		previous, err := m.state.payloads.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not get ancestor paylooad (%x): %w", ancestorID, err)
		}
		for _, guarantee := range previous.Guarantees {
			guarID := guarantee.ID()
			_, duplicated := gLookup[guarID]
			if duplicated {
				duplicateGuarIDs = append(duplicateGuarIDs, guarID)
			}
		}
		for _, seal := range previous.Seals {
			sealID := seal.ID()
			_, duplicated := sLookup[sealID]
			if duplicated {
				duplicateSealIDs = append(duplicateSealIDs, sealID)
			}
		}
		ancestorID = ancestor.ParentID
	}
	if len(duplicateGuarIDs) > 0 || len(duplicateSealIDs) > 0 {
		return fmt.Errorf("duplicate payload entities (guarantees: %s, seals: %s)", duplicateGuarIDs, duplicateSealIDs)
	}

	// FOURTH: Check that we can create a valid extension chain from the last
	// sealed block through all the seals included in the payload.

	// In order to accomplish this:
	// 1) Map each seal in the payload to the parent of the sealed block.
	// 2) Go through ancestors until we find last sealed block.
	// 3) Try to build a chain of seals by looking up by parent of sealed block.
	// We either succeed or end up with unused seals or an error.
	byParent := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range payload.Seals {
		sealed, err := m.state.headers.ByBlockID(seal.BlockID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed header: %w", err)
		}
		byParent[sealed.ParentID] = seal
	}
	if len(payload.Seals) > len(byParent) {
		return fmt.Errorf("multiple seals have the same parent block")
	}
	sealedID := candidate.ParentID
	var lastSeal *flow.Seal
	for {
		var err error
		lastSeal, err = m.state.seals.BySealedID(sealedID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("could not look up seal for block: %w", err)
		}
		break
	}
	for len(byParent) > 0 {
		seal, found := byParent[lastSeal.BlockID]
		if !found {
			return fmt.Errorf("could not find connecting seal (parent: %x)", lastSeal.BlockID)
		}
		if !bytes.Equal(seal.InitialState, lastSeal.FinalState) {
			return fmt.Errorf("seal execution states do not connect")
		}
		delete(byParent, lastSeal.BlockID)
		lastSeal = seal
	}

	// FIFTH: Map each block to the seal that sealed ID and index the state
	// commitment after the block. Also create an empty child lookup entry.
	err = operation.RetryOnConflict(m.state.db.Update, func(tx *badger.Txn) error {
		for _, seal := range payload.Seals {
			err = operation.IndexBlockSeal(seal.BlockID, seal.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not index block seal (block: %x): %w", seal.BlockID, err)
			}
		}
		err = operation.IndexStateCommitment(candidate.ID(), lastSeal.FinalState)(tx)
		if err != nil {
			return fmt.Errorf("could not index commit: %w", err)
		}
		err = operation.InsertBlockChildren(candidate.ID(), nil)(tx)
		if err != nil {
			return fmt.Errorf("could not insert empty block children: %w", err)
		}
		return nil
	})

	return nil
}

func (m *Mutator) Finalize(blockID flow.Identifier) error {

	// retrieve the finalized height
	var finalized uint64
	err := m.state.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized height: %w", err)
	}

	// get the finalized block ID
	var finalID flow.Identifier
	err = m.state.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return fmt.Errorf("could not retrieve finalized header: %w", err)
	}

	// retrieve the pending block
	pending, err := m.state.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve pending header: %w", err)
	}

	// Check to see if we are trying to finalize at a heigth that is already
	// finalized; if so, compary the ID of the finalized block.
	// 1) If it matches, this is a no-op.
	// 2) If it doesn't match, this is an invalid operation.
	if pending.Height <= finalized {
		dup, err := m.state.headers.ByHeight(pending.Height)
		if err != nil {
			return fmt.Errorf("could not get non-pending alternative: %w", err)
		}
		if dup.ID() != blockID {
			return fmt.Errorf("cannot finalize conflicting state (height: %d, pending: %x, finalized: %x)", pending.Height, blockID, dup.ID())
		}
		return nil
	}

	// in order to validate the validity of all changes, we need to iterate
	// through the blocks that need to be finalized from oldest to youngest;
	// we thus start at the youngest and remember the intermediary steps
	// while tracing back until we reach the finalized state
	pendings := []*flow.Header{pending}

	// create a copy of header for the loop to not change the header the slice above points to
	ancestorID := pending.ParentID
	for ancestorID != finalID {
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve parent (%x): %w", ancestorID, err)
		}
		pendings = append(pendings, ancestor)
		ancestorID = ancestor.ParentID
	}

	// now we can step backwards in order to go from oldest to youngest; for
	// each header, we reconstruct the block and then apply the related
	// changes to the protocol state
	for i := len(pendings) - 1; i >= 0; i-- {

		// Finalize the block
		err = m.state.db.Update(procedure.FinalizeBlock(pendings[i].ID()))
		if err != nil {
			return fmt.Errorf("could not finalize block (%s): %w", pendings[i].ID(), err)
		}
	}

	return nil
}
