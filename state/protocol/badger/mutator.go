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

func (m *Mutator) Bootstrap(commit flow.StateCommitment, genesis *flow.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// FIRST: execute all the validity checks on the genesis block

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

		// SECOND: update the underyling database with the genesis data

		// 1) insert the block, the genesis identities and index it by beight
		err := m.state.blocks.Store(genesis)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}
		err = operation.IndexBlockHeight(0, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not initialize boundary: %w", err)
		}
		err = operation.InsertBlockChildren(genesis.ID(), nil)(tx)
		if err != nil {
			return fmt.Errorf("could not insert empty block children: %w", err)
		}

		// TODO: put seal into payload to have it signed

		// 2) generate genesis execution result, insert and index by block
		result := flow.ExecutionResult{ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: flow.ZeroID,
			BlockID:          genesis.ID(),
			FinalStateCommit: commit,
		}}
		err = operation.InsertExecutionResult(&result)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis result: %w", err)
		}
		err = operation.IndexExecutionResult(genesis.ID(), result.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis result: %w", err)
		}

		// 3) generate genesis block seal, insert and index by block
		seal := flow.Seal{
			BlockID:      genesis.ID(),
			ResultID:     result.ID(),
			InitialState: flow.GenesisStateCommitment,
			FinalState:   result.FinalStateCommit,
		}
		err = operation.InsertSeal(seal.ID(), &seal)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis seal: %w", err)
		}
		err = operation.IndexBlockSeal(genesis.ID(), seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index genesis block seal: %w", err)
		}

		// 4) initialize all of the special views and heights
		err = operation.InsertStartedView(genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertVotedView(genesis.Header.View)(tx)
		if err != nil {
			return fmt.Errorf("could not insert started view: %w", err)
		}
		err = operation.InsertFinalizedHeight(genesis.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert finalized height: %w", err)
		}
		err = operation.InsertSealedHeight(genesis.Header.Height)(tx)
		if err != nil {
			return fmt.Errorf("could not insert sealed height: %w", err)
		}

		m.state.metrics.BlockProposed(genesis)
		m.state.metrics.BlockFinalized(genesis)
		m.state.metrics.BlockSealed(genesis)

		return nil
	})
}

func (m *Mutator) Extend(candidate *flow.Block) error {

	// FIRST: We do some initial cheap sanity checks. Currently, only the
	// genesis block can contain identities. We also want to make sure that the
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

	parent, err := m.state.headers.ByBlockID(candidate.Header.ParentID)
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
	limit := header.Height - flow.DefaultTransactionExpiry
	if limit > header.Height { // overflow check
		limit = 0
	}

	// build a list of all previously used guarantees on this part of the chain
	ancestorID = header.ParentID
	lookup := make(map[flow.Identifier]struct{})
	for {
		ancestor, err := m.state.headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor header (%x): %w", ancestorID, err)
		}
		payload, err := m.state.payloads.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not retrieve ancestor payload (%x): %w", ancestorID, err)
		}
		for _, guarantee := range payload.Guarantees {
			lookup[guarantee.ID()] = struct{}{}
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

	// FIFTH: For compliance of the seal payload, we don't need to check if they
	// were previously included; each seal can only refer to a single unique
	// block. Instead, we see if we can build a valid chain of seals from the
	// seal of the parent block that uses all of the payload seals.

	// we create a map that allows us to look up seals by the parent of the
	// block that was sealed, which allows us to look up the chain starting at
	// the parent of the candidate block
	byParent := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range payload.Seals {
		sealed, err := m.state.headers.ByBlockID(seal.BlockID)
		if err != nil {
			return fmt.Errorf("could not retrieve sealed header (%x): %w", seal.BlockID, err)
		}
		if sealed.Height <= limit {
			return fmt.Errorf("sealed blocks go back too far (sealed: %d, limit: %d)", sealed.Height, limit)
		}
		byParent[sealed.ParentID] = seal
	}
	if len(payload.Seals) > len(byParent) {
		return fmt.Errorf("multiple seals have the same parent block")
	}

	// starting at the parent seal, we then try to build a chain of seals that
	// validly extends the execution state, using up all of the seals
	lastSeal, err := m.state.seals.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not look up parent seal (%x): %w", header.ParentID, err)
	}
	for len(byParent) > 0 {
		seal, connected := byParent[lastSeal.BlockID]
		if !connected {
			return fmt.Errorf("could not find connecting seal (parent: %x)", lastSeal.BlockID)
		}
		if !bytes.Equal(lastSeal.FinalState, seal.InitialState) {
			return fmt.Errorf("seal execution states do not connect")
		}
		delete(byParent, lastSeal.BlockID)
		lastSeal = seal
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
		err := operation.IndexBlockSeal(blockID, lastSeal.ID())(tx)
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

	// SEVENTH: Metrics.

	m.state.metrics.BlockProposed(candidate)

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
	pending, err := m.state.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve pending header: %w", err)
	}
	if pending.ParentID != finalID {
		return fmt.Errorf("can only finalized child of last finalized block")
	}

	// SECOND: We also want to update the last sealed height. Retrieve the block
	// seal indexed for the block and retrieve the block that was sealed by it.

	seal, err := m.state.seals.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not look up sealed header: %w", err)
	}
	sealed, err := m.state.headers.ByBlockID(seal.BlockID)
	if err != nil {
		return fmt.Errorf("could not retrieve sealed heder: %w", err)
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
		err = operation.IndexBlockHeight(pending.Height, blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not insert number mapping: %w", err)
		}
		err = operation.UpdateFinalizedHeight(pending.Height)(tx)
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
