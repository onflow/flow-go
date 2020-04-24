// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
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

func (m *Mutator) Bootstrap(genesis *flow.Block) error {
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

		return procedure.Bootstrap(genesis)(tx)
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// retrieve the block
		var block flow.Block
		err := procedure.RetrieveBlock(blockID, &block)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		// retrieve the seal for the parent
		var parentSeal flow.Seal
		err = procedure.LookupSealByBlock(block.ParentID, &parentSeal)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent seal: %w", err)
		}

		// check the header validity
		err = checkExtendHeader(tx, &block.Header)
		if err != nil {
			return fmt.Errorf("extend header not valid: %w", err)
		}

		// check the payload validity
		err = checkExtendPayload(tx, &block)
		if err != nil {
			return fmt.Errorf("extend payload not valid: %w", err)
		}

		// check the block integrity
		if block.Payload.Hash() != block.Header.PayloadHash {
			return fmt.Errorf("block integrity check failed")
		}

		// TODO: update the stakes with the stake deltas

		// create a lookup for each seal by parent
		lookup := make(map[string]*flow.Seal, len(block.Seals))
		for _, seal := range block.Seals {
			lookup[string(seal.InitialState)] = seal
		}

		// starting with what was the state commitment at the parent block, we
		// match each seal into the chain of commits
		nextSeal := &parentSeal
		for len(lookup) > 0 {

			// first check if we have a seal connecting to current latest commit
			possibleNextSeal, ok := lookup[string(nextSeal.FinalState)]
			if !ok {
				return fmt.Errorf("seals not connected to state chain (%x)", nextSeal.FinalState)
			}

			// delete matched seal from lookup and forward to point to seal commit
			delete(lookup, string(nextSeal.FinalState))
			nextSeal = possibleNextSeal
		}

		// insert the the seal into our seals timeline
		err = operation.IndexSealIDByBlock(blockID, nextSeal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index seal by block: %w", err)
		}

		return nil
	})
}

func (m *Mutator) Finalize(blockID flow.Identifier) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// retrieve the block to make sure we have it
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		// retrieve the current finalized state boundary
		var boundary uint64
		err = operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the hash of the boundary
		var headID flow.Identifier
		err = operation.RetrieveNumber(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// in order to validate the validity of all changes, we need to iterate
		// through the blocks that need to be finalized from oldest to youngest;
		// we thus start at the youngest remember all of the intermediary steps
		// while tracing back until we reach the finalized state
		headers := []*flow.Header{&header}

		// create a copy of header for the loop to not change the header the slice above points to
		loopHeader := header
		for loopHeader.ParentID != headID {
			var retrievedHeader flow.Header
			err = operation.RetrieveHeader(loopHeader.ParentID, &retrievedHeader)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve parent (%x): %w", header.ParentID, err)
			}
			headers = append(headers, &retrievedHeader)
			loopHeader = retrievedHeader
		}

		// now we can step backwards in order to go from oldest to youngest; for
		// each header, we reconstruct the block and then apply the related
		// changes to the protocol state
		for i := len(headers) - 1; i >= 0; i-- {

			// Finalize the block
			err = procedure.FinalizeBlock(headers[i].ID())(tx)
			if err != nil {
				return fmt.Errorf("could not finalize block (%s): %w", header.ID(), err)
			}
		}

		return nil
	})
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

	// we should have one seal
	if len(payload.Seals) != 1 {
		return fmt.Errorf("genesis block must have one seal")
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

	// check the one seal
	seal := payload.Seals[0]

	// seal should have zero ID as parent block
	if seal.BlockID != flow.ZeroID {
		return fmt.Errorf("initial seal needs zero block ID")
	}

	// seal should have zero ID as parent state commit
	if seal.InitialState != nil {
		return fmt.Errorf("initial seal needs nil parent commit")
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

func checkExtendHeader(tx *badger.Txn, header *flow.Header) error {

	// get the boundary number of the finalized state
	var boundary uint64
	err := operation.RetrieveBoundary(&boundary)(tx)
	if err != nil {
		return fmt.Errorf("could not get boundary: %w", err)
	}

	// get the hash of the latest finalized block
	var finalID flow.Identifier
	err = operation.RetrieveNumber(boundary, &finalID)(tx)
	if err != nil {
		return fmt.Errorf("could not retrieve hash: %w", err)
	}

	// trace back from new block until we find a block that has the latest
	// finalized block as its parent
	height := header.Height
	ancestorID := header.ParentID
	for ancestorID != finalID {

		// get the parent of the block we current look at
		var ancestor flow.Header
		err = operation.RetrieveHeader(ancestorID, &ancestor)(tx)
		if err != nil {
			return fmt.Errorf("could not get block's ancestor from state (%x): %w", ancestorID, err)
		}

		// check that the parent is one less in height than previous block; this
		// is redundant for all but the first check, but cheap, and makes the
		// code a lot simpler
		if height != ancestor.Height+1 {
			return fmt.Errorf("block needs height equal to ancestor height+1 (%d != %d+1)", height, ancestor.Height)
		}

		// check if the ancestor is unfinalized, but already behind the last finalized height (orphaned fork)
		if ancestor.Height < boundary {
			return fmt.Errorf("block doesn't connect to finalized state (%d < %d), ancestorID (%v)", ancestor.Height, boundary, ancestorID)
		}

		// forward to next parent
		ancestorID = ancestor.ParentID
		height = ancestor.Height
	}

	return nil
}

func checkExtendPayload(tx *badger.Txn, block *flow.Block) error {

	// currently we don't support identities except for genesis block
	if len(block.Payload.Identities) > 0 {
		return fmt.Errorf("extend block has identities")
	}

	// we check contents for duplicates from the parent height and ID
	height := block.Header.Height - 1
	blockID := block.Header.ParentID

	// check we have no duplicate guarantees
	err := operation.VerifyGuaranteePayload(height, blockID, flow.GetIDs(block.Payload.Guarantees))(tx)
	if errors.Is(err, storage.ErrAlreadyIndexed) {
		return fmt.Errorf("found duplicate guarantee in payload: %w", err)
	}
	if err != nil {
		return fmt.Errorf("could not verify guarantee payload: %w", err)
	}

	// check we have no duplicate block seals
	err = operation.VerifySealPayload(height, blockID, flow.GetIDs(block.Payload.Seals))(tx)
	if errors.Is(err, storage.ErrAlreadyIndexed) {
		return fmt.Errorf("found duplicate seal in payload: %w", err)
	}
	if err != nil {
		return fmt.Errorf("could not verify seal payload: %w", err)
	}

	return nil
}
