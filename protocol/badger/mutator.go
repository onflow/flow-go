// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(genesis *flow.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// check that the new identities are valid
		err := checkIdentitiesValidity(tx, genesis.Identities)
		if err != nil {
			return fmt.Errorf("could not check identities validity: %w", err)
		}

		// initialize the boundary of the finalized state
		err = initializeFinalizedBoundary(tx, genesis)
		if err != nil {
			return fmt.Errorf("could not initialize finalized boundary: %w", err)
		}

		// store the block contents in the database
		err = storeBlockContents(tx, genesis)
		if err != nil {
			return fmt.Errorf("could not insert block payload: %w", err)
		}

		// apply the block changes to the finalized state
		err = applyBlockChanges(tx, genesis)
		if err != nil {
			return fmt.Errorf("could not insert block deltas: %w", err)
		}

		return nil
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// retrieve the identities
		var identities flow.IdentityList
		err := operation.RetrieveIdentities(blockID, &identities)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve identities: %w", err)
		}

		// check that the new identities are valid
		err = checkIdentitiesValidity(tx, identities)
		if err != nil {
			return fmt.Errorf("could not check identities validity: %w", err)
		}

		// get the block header to check
		var header flow.Header
		err = operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// check that the block is a valid extension of the protocol state
		err = checkHeaderValidity(tx, &header)
		if err != nil {
			return fmt.Errorf("could not check block validity: %w", err)
		}

		// TODO: stuff like indexing identities etc

		return nil
	})
}

type step struct {
	blockID flow.Identifier
	header  flow.Header
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
		err = operation.RetrieveBlockID(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// in order to validate the validity of all changes, we need to iterate
		// through the blocks that need to be finalized from oldest to youngest;
		// we thus start at the youngest remember all of the intermediary steps
		// while tracing back until we reach the finalized state
		steps := []step{{blockID: blockID, header: header}}
		for header.ParentID != headID {
			blockID = header.ParentID
			err = operation.RetrieveHeader(header.ParentID, &header)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve parent (%x): %w", header.ParentID, err)
			}
			steps = append(steps, step{blockID: blockID, header: header})
		}

		// now we can step backwards in order to go from oldest to youngest; for
		// each header, we reconstruct the block and then apply the related
		// changes to the protocol state
		var identities flow.IdentityList
		var guarantees []*flow.CollectionGuarantee
		for i := len(steps) - 1; i >= 0; i-- {

			// get the identities
			s := steps[i]
			err = operation.RetrieveIdentities(s.blockID, &identities)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve identities (%x): %w", s.blockID, err)
			}

			// get the collection guarantees
			err = operation.RetrieveGuarantees(s.blockID, &guarantees)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve guarantees (%x): %w", s.blockID, err)
			}

			// create contant
			content := flow.Content{
				Identities: identities,
				Guarantees: guarantees,
			}

			// reconstruct block
			block := flow.Block{
				Header:  header,
				Payload: content.Payload(),
				Content: content,
			}

			// insert the deltas
			err = applyBlockChanges(tx, &block)
			if err != nil {
				return fmt.Errorf("could not insert block deltas (%x): %w", s.blockID, err)
			}
		}

		return nil
	})
}

func checkIdentitiesValidity(tx *badger.Txn, identities flow.IdentityList) error {

	// check that we don't have duplicate identity entries
	lookup := make(map[flow.Identifier]struct{})
	for _, id := range identities {
		_, ok := lookup[id.NodeID]
		if ok {
			return fmt.Errorf("duplicate node identity (%x)", id.NodeID)
		}
		lookup[id.NodeID] = struct{}{}
	}

	// for each identity, check it has a non-zero stake
	for _, id := range identities {
		if id.Stake == 0 {
			return fmt.Errorf("invalid zero stake (%x)", id.NodeID)
		}
	}

	// for each identity, check it doesn't have a role yet
	for _, id := range identities {

		// check for role
		var role flow.Role
		err := operation.RetrieveRole(id.NodeID, &role)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}

		if err == nil {
			return fmt.Errorf("identity role already exists (%x: %s)", id.NodeID, role)
		}
		return fmt.Errorf("could not check identity role (%x): %w", id.NodeID, err)
	}

	// for each identity, check it doesn't have an address yet
	for _, id := range identities {

		// check for address
		var address string
		err := operation.RetrieveAddress(id.NodeID, &address)(tx)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}

		if err == nil {
			return fmt.Errorf("identity address already exists (%x: %s)", id.NodeID, address)
		}
		return fmt.Errorf("could not check identity address (%x): %w", id.NodeID, err)
	}

	return nil
}

func checkHeaderValidity(tx *badger.Txn, header *flow.Header) error {

	// get the boundary number of the finalized state
	var boundary uint64
	err := operation.RetrieveBoundary(&boundary)(tx)
	if err != nil {
		return fmt.Errorf("could not get boundary: %w", err)
	}

	// get the hash of the latest finalized block
	var headID flow.Identifier
	err = operation.RetrieveBlockID(boundary, &headID)(tx)
	if err != nil {
		return fmt.Errorf("could not retrieve hash: %w", err)
	}

	// get the first parent of the introduced block to check the number
	var parent flow.Header
	err = operation.RetrieveHeader(header.ParentID, &parent)(tx)
	if err != nil {
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// if new block number has a lower number, we can't finalize it
	if header.Number <= parent.Number {
		return fmt.Errorf("block needs higher nummber (%d <= %d)", header.Number, parent.Number)
	}

	// NOTE: in the default case, the first parent is the boundary, se we don't
	// load the first parent twice almost ever; even in cases where we do, we
	// badger has efficietn caching, so no reason to complicate the algorithm
	// here to try avoiding one extra header loading

	// trace back from new block until we find a block that has the latest
	// finalized block as its parent
	for header.ParentID != headID {

		// get the parent of current block
		err = operation.RetrieveHeader(header.ParentID, header)(tx)
		if err != nil {
			return fmt.Errorf("could not get parent (%x): %w", header.ParentID, err)
		}

		// if its number is below current boundary, the block does not connect
		// to the finalized protocol state and would break database consistency
		if header.Number < boundary {
			return fmt.Errorf("block doesn't connect to finalized state")
		}

	}

	return nil
}

func initializeFinalizedBoundary(tx *badger.Txn, genesis *flow.Block) error {

	// the initial finalized boundary needs to be height zero
	if genesis.Number != 0 {
		return fmt.Errorf("invalid initial finalized boundary (%d != 0)", genesis.Number)
	}

	// the parent must be zero hash
	if genesis.ParentID != flow.ZeroID {
		return errors.New("genesis parent must be zero hash")
	}

	// genesis should have no collections
	if len(genesis.Guarantees) > 0 {
		return errors.New("genesis should not contain collections")
	}

	// insert the initial finalized state boundary
	err := operation.InsertBoundary(genesis.Number)(tx)
	if err != nil {
		return fmt.Errorf("could not insert boundary: %w", err)
	}

	return nil
}

func storeBlockContents(tx *badger.Txn, block *flow.Block) error {

	// insert the header into the DB
	err := operation.InsertHeader(&block.Header)(tx)
	if err != nil {
		return fmt.Errorf("could not insert header: %w", err)
	}

	// NOTE: we might to improve this to insert an index, and then insert each
	// entity separately; this would allow us to retrieve the entities one by
	// one, instead of only by block

	// insert the identities into the DB
	err = operation.InsertIdentities(block.ID(), block.Identities)(tx)
	if err != nil {
		return fmt.Errorf("could not insert identities: %w", err)
	}

	// insert the collection guarantees into the DB
	for _, cg := range block.Guarantees {
		err = operation.InsertGuarantee(cg)(tx)
		if err != nil {
			return fmt.Errorf("could not insert collection guarantee: %w", err)
		}
		err = operation.IndexGuarantee(block.ID(), cg)(tx)
		if err != nil {
			return fmt.Errorf("could not index collection guarantee: %w", err)
		}
	}

	return nil
}

func applyBlockChanges(tx *badger.Txn, block *flow.Block) error {

	// insert the height to hash mapping for finalized block
	err := operation.InsertBlockID(block.Number, block.ID())(tx)
	if err != nil {
		return fmt.Errorf("could not insert hash: %w", err)
	}

	// update the finalized boundary number
	err = operation.UpdateBoundary(block.Number)(tx)
	if err != nil {
		return fmt.Errorf("could not update boundary: %w", err)
	}

	// insert the information for each new identity
	for _, id := range block.Identities {

		// insert the role
		err := operation.InsertRole(id.NodeID, id.Role)(tx)
		if err != nil {
			return fmt.Errorf("could not insert role (%x): %w", id.NodeID, err)
		}

		// insert the address
		err = operation.InsertAddress(id.NodeID, id.Address)(tx)
		if err != nil {
			return fmt.Errorf("could not insert address (%x): %w", id.NodeID, err)
		}

		// insert the stake delta
		err = operation.InsertDelta(block.Number, id.Role, id.NodeID, int64(id.Stake))(tx)
		if err != nil {
			return fmt.Errorf("could not insert delta (%x): %w", id.NodeID, err)
		}
	}

	return nil
}
