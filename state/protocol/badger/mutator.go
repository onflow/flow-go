// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(commit flow.StateCommitment, genesis *flow.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// check that the genesis block is valid
		err := checkGenesisHeader(&genesis.Header)
		if err != nil {
			return fmt.Errorf("genesis header not valid: %w", err)
		}

		// check that the new identities are valid
		err = checkGenesisPayload(tx, &genesis.Payload)
		if err != nil {
			return fmt.Errorf("genesis identities not valid: %w", err)
		}

		return procedure.Bootstrap(commit, genesis)(tx)
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {

	// sealed state we want to index for the block
	var lastSeal flow.Seal

	// check whether the block is a valid extension of the state
	err := m.state.db.Update(func(tx *badger.Txn) error {

		// 1) check that the introduced block directly connects to the last
		// finalized state; otherwise, it can never become finalized:
		// - get the header that is the state extension candidate
		// - get the header that is the last finalized header
		// - trace back from candidate through parents until we either:
		// a) hit the last finalized block; we connect to the last finalized
		// state and the candidate is potentially valid
		// b) we hit a block that has lower height than the last finalized
		// block; we thus don't connect to the latest finalized state and the
		// candidate can not be valid

		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		var boundary uint64
		err = operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not get boundary: %w", err)
		}

		var finalID flow.Identifier
		err = operation.RetrieveNumber(boundary, &finalID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve hash: %w", err)
		}

		aheight := header.Height
		ancestorID := header.ParentID
		for ancestorID != finalID {

			var ancestor flow.Header
			err = operation.RetrieveHeader(ancestorID, &ancestor)(tx)
			if err != nil {
				return fmt.Errorf("could not get block's ancestor from state (%x): %w", ancestorID, err)
			}

			// NOTE: we check for each ancestor if the height is one higher than
			// the parent; we really just need to check for the candidate, but
			// the check is cheap enough to do it for all of them
			if aheight != ancestor.Height+1 {
				return fmt.Errorf("block needs height equal to ancestor height+1 (%d != %d+1)", aheight, ancestor.Height)
			}

			// if this check is true, we are not connected to the finalized
			// state, but instead (potentially) to an ancestor of the finalized
			// state that already had a valid child finalized
			if ancestor.Height < boundary {
				return fmt.Errorf("block doesn't connect to finalized state (%d < %d), ancestorID (%v)", ancestor.Height, boundary, ancestorID)
			}

			// if we have neither hit the last finalized block, nor a block that
			// is below its height, we forward our pointer to the next ancestor
			ancestorID = ancestor.ParentID
			aheight = ancestor.Height
		}

		// 2) check whether the block has guarantees that were already included
		// in a previous finalized block on this chain

		var guaranteeIDs []flow.Identifier
		err = operation.LookupGuaranteePayload(header.Height, blockID, header.ParentID, &guaranteeIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not look up guarantees: %w", err)
		}
		for _, guaranteeID := range guaranteeIDs {
			old := m.state.pcache.HasGuarantee(guaranteeID)
			if old {
				return fmt.Errorf("guarantee already finalized (%x)", guaranteeID)
			}
		}

		// 3) check whether the block has seals that were already included in a
		// a previous block on this chain

		var sealIDs []flow.Identifier
		err = operation.LookupSealPayload(header.Height, blockID, header.ParentID, &sealIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not look up seals: %w", err)
		}
		for _, sealID := range sealIDs {
			old := m.state.pcache.HasSeal(sealID)
			if old {
				return fmt.Errorf("seal already finalized (%x)", sealID)
			}
		}

		// 4) check that all seals validly connect to the last sealed state

		// NOTE: we keep the following loop to simulate real performance, even
		// if the check is currently useless, as we never include seals

		// TODO: re-enable after we want to include seals
		sealIDs = nil

		// create a lookup for all seals by the parent of the block they sealed
		byParent := make(map[flow.Identifier]*flow.Seal)
		for _, sealID := range sealIDs {
			var seal flow.Seal
			err = operation.RetrieveSeal(sealID, &seal)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve seal: %w", err)
			}
			var header flow.Header
			err = operation.RetrieveHeader(seal.BlockID, &header)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve sealed header: %w", err)
			}
			_, already := byParent[header.ParentID]
			if already {
				return fmt.Errorf("duplicate block seal (sealed: %x, parent: %x)", seal.BlockID, header.ParentID)
			}
			byParent[header.ParentID] = &seal
		}

		// start at the parent seal to extend execution state
		err = procedure.LookupSealByBlock(header.ParentID, &lastSeal)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent seal: %w", err)
		}

		// we keep connecting seals from the map until they are all gone or we
		// have errored
		for len(byParent) > 0 {

			// get a seal that has the last sealed block as parent
			seal, found := byParent[lastSeal.BlockID]
			if !found {
				return fmt.Errorf("could not find connecting seal (parent: %x)", lastSeal.BlockID)
			}

			// check if the seal connects to the last known execution state
			if !bytes.Equal(seal.InitialState, lastSeal.FinalState) {
				return fmt.Errorf("seal execution states do not connect")
			}

			// delete the seal from the map and forward pointer
			delete(byParent, lastSeal.BlockID)
			lastSeal = *seal
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not check extension: %w", err)
	}

	// execute the write-portion of the extension (with retry)
	err = operation.RetryOnConflict(func() error {
		return m.state.db.Update(func(tx *badger.Txn) error {
			err = operation.IndexSealIDByBlock(blockID, lastSeal.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not index seal for extension: %w", err)
			}
			err = operation.IndexStateCommitment(blockID, lastSeal.FinalState)(tx)
			if err != nil {
				return fmt.Errorf("could not index state commitment: %w", err)
			}
			return nil
		})
	})

	return nil
}

func checkGenesisHeader(header *flow.Header) error {
	// the initial height needs to be height zero
	if header.Height != 0 {
		return fmt.Errorf("invalid initial height (%d != 0)", header.Height)
	}

	// the parent must be zero hash
	if header.ParentID != flow.ZeroID {
		return errors.New("genesis parent must be zero hash")
	}

	return nil
}

func checkGenesisPayload(tx *badger.Txn, payload *flow.Payload) error {

	// we should have no guarantees
	if len(payload.Guarantees) > 0 {
		return fmt.Errorf("genesis block must have zero guarantees")
	}

	// we should have no seals
	if len(payload.Seals) > 0 {
		return fmt.Errorf("genesis block must have zero seals")
	}

	// we should have one role of each type at least
	roles := make(map[flow.Role]uint)
	for _, identity := range payload.Identities {
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
	for _, identity := range payload.Identities {
		_, ok := identLookup[identity.NodeID]
		if ok {
			return fmt.Errorf("duplicate node identifier (%x)", identity.NodeID)
		}
		identLookup[identity.NodeID] = struct{}{}
	}

	// check identities do not have duplicate addresses
	addrLookup := make(map[string]struct{})
	for _, identity := range payload.Identities {
		_, ok := addrLookup[identity.Address]
		if ok {
			return fmt.Errorf("duplicate node address (%x)", identity.Address)
		}
		addrLookup[identity.Address] = struct{}{}
	}

	// for each identity, check it has a non-zero stake
	for _, identity := range payload.Identities {
		if identity.Stake == 0 {
			return fmt.Errorf("zero stake identity (%x)", identity.NodeID)
		}
	}

	return nil
}
