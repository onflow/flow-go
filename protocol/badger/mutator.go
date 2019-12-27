// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
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
		err := checkIdentitiesValidity(tx, genesis.NewIdentities)
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

func (m *Mutator) Extend(block *flow.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// check that the new identities are valid
		err := checkIdentitiesValidity(tx, block.NewIdentities)
		if err != nil {
			return fmt.Errorf("could not check identities validity: %w", err)
		}

		// check that the block is a valid extension of the protocol state
		err = checkBlockValidity(tx, block.Header)
		if err != nil {
			return fmt.Errorf("could not check block validity: %w", err)
		}

		// store the block contents in the database
		err = storeBlockContents(tx, block)
		if err != nil {
			return fmt.Errorf("could not insert block payload: %w", err)
		}

		return nil
	})
}

type step struct {
	hash   crypto.Hash
	header flow.Header
}

func (m *Mutator) Finalize(hash crypto.Hash) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// retrieve the block to make sure we have it
		var header flow.Header
		err := operation.RetrieveHeader(hash, &header)(tx)
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
		var head crypto.Hash
		err = operation.RetrieveHash(boundary, &head)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// in order to validate the validity of all changes, we need to iterate
		// through the blocks that need to be finalized from oldest to youngest;
		// we thus start at the youngest remember all of the intermediary steps
		// while tracing back until we reach the finalized state
		steps := []step{{hash: hash, header: header}}
		for !header.Parent.Equal(head) {
			hash = header.Parent
			err = operation.RetrieveHeader(header.Parent, &header)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve parent (%x): %w", header.Parent, err)
			}
			steps = append(steps, step{hash: hash, header: header})
		}

		// now we can step backwards in order to go from oldest to youngest; for
		// each header, we reconstruct the block and then apply the related
		// changes to the protocol state
		var identities flow.IdentityList
		var collections []*flow.GuaranteedCollection
		for i := len(steps) - 1; i >= 0; i-- {

			// get the identities
			s := steps[i]
			err = operation.RetrieveIdentities(s.hash, &identities)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve identities (%x): %w", s.hash, err)
			}

			// get the guaranteed collections
			err = operation.RetrieveCollections(s.hash, &collections)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve collections (%x): %w", s.hash, err)
			}

			// reconstruct block
			block := flow.Block{
				Header:                header,
				NewIdentities:         identities,
				GuaranteedCollections: collections,
			}

			// insert the deltas
			err = applyBlockChanges(tx, &block)
			if err != nil {
				return fmt.Errorf("could not insert block deltas (%x): %w", s.hash, err)
			}
		}

		return nil
	})
}

func checkIdentitiesValidity(tx *badger.Txn, identities []flow.Identity) error {

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
		if err == storage.NotFoundErr {
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
		if err == storage.NotFoundErr {
			continue
		}
		if err == nil {
			return fmt.Errorf("identity address already exists (%x: %s)", id.NodeID, address)
		}
		return fmt.Errorf("could not check identity address (%x): %w", id.NodeID, err)
	}

	return nil
}

func checkBlockValidity(tx *badger.Txn, header flow.Header) error {

	// get the boundary number of the finalized state
	var boundary uint64
	err := operation.RetrieveBoundary(&boundary)(tx)
	if err != nil {
		return fmt.Errorf("could not get boundary: %w", err)
	}

	// get the hash of the latest finalized block
	var head crypto.Hash
	err = operation.RetrieveHash(boundary, &head)(tx)
	if err != nil {
		return fmt.Errorf("could not retrieve hash: %w", err)
	}

	// get the first parent of the introduced block to check the number
	var parent flow.Header
	err = operation.RetrieveHeader(header.Parent, &parent)(tx)
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
	for !header.Parent.Equal(head) {

		// get the parent of current block
		err = operation.RetrieveHeader(header.Parent, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not get parent (%x): %w", header.Parent, err)
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
	if !bytes.Equal(genesis.Parent, crypto.ZeroHash) {
		return errors.New("genesis parent must be zero hash")
	}

	// genesis should have no collections
	if len(genesis.GuaranteedCollections) > 0 {
		return errors.New("genesis should not contain collections")
	}

	// insert the initial finalized state boundary
	err := operation.InsertNewBoundary(genesis.Number)(tx)
	if err != nil {
		return fmt.Errorf("could not insert boundary: %w", err)
	}

	return nil
}

func storeBlockContents(tx *badger.Txn, block *flow.Block) error {

	// insert the header into the DB
	err := operation.InsertNewHeader(&block.Header)(tx)
	if err != nil {
		return fmt.Errorf("could not insert header: %w", err)
	}

	// NOTE: we might to improve this to insert an index, and then insert each
	// entity separately; this would allow us to retrieve the entities one by
	// one, instead of only by block

	// insert the identities into the DB
	err = operation.InsertNewIdentities(block.Hash(), block.NewIdentities)(tx)
	if err != nil {
		return fmt.Errorf("could not insert identities: %w", err)
	}

	// insert the guaranteed collections into the DB
	err = operation.InsertNewCollections(block.Hash(), block.GuaranteedCollections)(tx)
	if err != nil {
		return fmt.Errorf("could not insert collections: %w", err)
	}

	return nil
}

func applyBlockChanges(tx *badger.Txn, block *flow.Block) error {

	// insert the height to hash mapping for finalized block
	err := operation.InsertNewHash(block.Number, block.Hash())(tx)
	if err != nil {
		return fmt.Errorf("could not insert hash: %w", err)
	}

	// update the finalized boundary number
	err = operation.UpdateBoundary(block.Number)(tx)
	if err != nil {
		return fmt.Errorf("could not update boundary: %w", err)
	}

	// insert the information for each new identity
	for _, id := range block.NewIdentities {

		// insert the role
		err := operation.InsertNewRole(id.NodeID, id.Role)(tx)
		if err != nil {
			return fmt.Errorf("could not insert role (%x): %w", id.NodeID, err)
		}

		// insert the address
		err = operation.InsertNewAddress(id.NodeID, id.Address)(tx)
		if err != nil {
			return fmt.Errorf("could not insert address (%x): %w", id.NodeID, err)
		}

		// insert the stake delta
		err = operation.InsertNewDelta(block.Number, id.Role, id.NodeID, int64(id.Stake))(tx)
		if err != nil {
			return fmt.Errorf("could not insert delta (%x): %w", id.NodeID, err)
		}
	}

	return nil
}
