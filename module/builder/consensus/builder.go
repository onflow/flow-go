// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	db         *badger.DB
	guarantees mempool.Guarantees
	seals      mempool.Seals
	cfg        Config
}

// NewBuilder creates a new block builder.
func NewBuilder(db *badger.DB, guarantees mempool.Guarantees, seals mempool.Seals, options ...func(*Config)) *Builder {

	// initialize default config
	cfg := Config{
		minInterval: 500 * time.Millisecond,
		maxInterval: 10 * time.Second,
	}

	// apply option parameters
	for _, option := range options {
		option(&cfg)
	}

	b := &Builder{
		db:         db,
		guarantees: guarantees,
		seals:      seals,
		cfg:        cfg,
	}
	return b
}

// BuildOn creates a new block header build on the provided parent, using the given view and applying the
// custom setter function to allow the caller to make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header)) (*flow.Header, error) {
	var header *flow.Header
	err := b.db.Update(func(tx *badger.Txn) error {

		// STEP ONE: get the payload entity IDs for all entities that are included
		// in ancestor blocks which are not finalized yet; this allows us to avoid
		// including them in a block on the same fork twice

		// first, we need to know what the latest finalized block number is
		var boundary uint64
		err := operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// get the last finalized block ID
		var finalizedID flow.Identifier
		err = operation.RetrieveNumber(boundary, &finalizedID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve finalized ID: %w", err)
		}

		// for each unfinalized ancestor of the payload we are building, we retrieve
		// a list of all pending IDs for guarantees and seals; we can use them to
		// exclude entities from being included in two blocks on the same fork.
		ancestorID := parentID
		guaranteeLookup := make(map[flow.Identifier]struct{})
		sealLookup := make(map[flow.Identifier]struct{})
		for {

			// retrieve the header for the ancestor
			var ancestor flow.Header
			err = operation.RetrieveHeader(ancestorID, &ancestor)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve ancestor (%x): %w", ancestorID, err)
			}

			// if we have reached the finalized boundary, stop indexing
			if ancestor.Height <= boundary {
				break
			}

			// look up the ancestor's guarantees
			var guaranteeIDs []flow.Identifier
			err = operation.LookupGuaranteePayload(ancestor.Height, ancestorID, ancestor.ParentID, &guaranteeIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not look up ancestor guarantees (%x): %w", ancestor.PayloadHash, err)
			}

			// look up the ancestor's seals
			var sealIDs []flow.Identifier
			err = operation.LookupSealPayload(ancestor.Height, ancestorID, ancestor.ParentID, &sealIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not look up ancestor seals (%x): %w", ancestor.PayloadHash, err)
			}

			// insert guarantees and seals into the lookups
			for _, guaranteeID := range guaranteeIDs {
				guaranteeLookup[guaranteeID] = struct{}{}
			}
			for _, sealID := range sealIDs {
				sealLookup[sealID] = struct{}{}
			}

			// continue with the next ancestor in the chain
			ancestorID = ancestor.ParentID
		}

		// STEP TWO: build a payload that includes as much of the memory pool
		// contents as possible, while remaining valid on the respective fork;
		// for guarantees, this implies they were not included in an unfinalized
		// ancestor yet; for block seals, it means they were not included yet
		// *and* they are a valid extension of the last valid execution state

		// collect guarantees from memory pool, excluding those already pending
		// on this fork
		var guaranteeIDs []flow.Identifier
		for _, guarantee := range b.guarantees.All() {
			_, ok := guaranteeLookup[guarantee.ID()]
			if ok {
				continue
			}
			guaranteeIDs = append(guaranteeIDs, guarantee.ID())
		}

		// find any guarantees that conflict with FINALIZED blocks
		var invalidIDs map[flow.Identifier]struct{}
		err = operation.CheckGuaranteePayload(boundary, finalizedID, guaranteeIDs, &invalidIDs)(tx)
		if err != nil {
			return fmt.Errorf("could not check guarantee payload: %w", err)
		}

		var guarantees []*flow.CollectionGuarantee
		for _, guaranteeID := range guaranteeIDs {

			_, isInvalid := invalidIDs[guaranteeID]
			if isInvalid {
				// remove from mempool, it will never be valid
				b.guarantees.Rem(guaranteeID)
				continue
			}

			// add ONLY non-conflicting guarantees to the final payload
			guarantee, err := b.guarantees.ByID(guaranteeID)
			if err != nil {
				return fmt.Errorf("could not get guarantee from pool: %w", err)
			}
			guarantees = append(guarantees, guarantee)
		}

		// get the finalized state commitment at the parent
		var seal flow.Seal
		err = procedure.LookupSealByBlock(parentID, &seal)(tx)
		if err != nil {
			return fmt.Errorf("could not get parent state commit: %w", err)
		}

		// we then keep adding seals that follow this state commit from the pool
		var seals []*flow.Seal

		// create a copy to avoid modifying when referenced in an array
		loopSeal := &seal
		for {
			// get a seal that extends the last known state commitment
			nextSeal, err := b.seals.ByPreviousState(loopSeal.FinalState)
			if errors.Is(err, mempool.ErrEntityNotFound) {
				break
			} else if bytes.Equal(nextSeal.PreviousState, loopSeal.FinalState) {
				// TODO Edgecase where no transactions happen in a block. Unlikely in mature system, but possible for tests
				// https://github.com/dapperlabs/flow-go/issues/3348
				break
			}
			if err != nil {
				return fmt.Errorf("could not get extending seal (%x): %w", loopSeal.FinalState, err)
			}

			// add the seal to our list and forward to the known last valid state
			seals = append(seals, nextSeal)
			loopSeal = nextSeal
		}

		seal = *loopSeal

		// STEP THREE: we have the guarantees and seals we can validly include
		// in the payload built on top of the given block. Now we need to build
		// and store the block header, as well as index the payload contents.

		// build the payload so we can get the hash
		payload := flow.Payload{
			Identities: nil,
			Guarantees: guarantees,
			Seals:      seals,
		}

		// retrieve the parent to set the height
		var parent flow.Header
		err = operation.RetrieveHeader(parentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		// calculate the timestamp and cutoffs
		timestamp := time.Now().UTC()
		from := parent.Timestamp.Add(b.cfg.minInterval)
		to := parent.Timestamp.Add(b.cfg.maxInterval)

		// adjust timestamp if outside of cutoffs
		if timestamp.Before(from) {
			timestamp = from
		}
		if timestamp.After(to) {
			timestamp = to
		}

		// construct default block on top of the provided parent
		header = &flow.Header{
			ChainID:     parent.ChainID,
			ParentID:    parentID,
			Height:      parent.Height + 1,
			Timestamp:   timestamp,
			PayloadHash: payload.Hash(),

			// the following fields should be set by the custom function as needed
			// NOTE: we could abstract all of this away into an interface{} field,
			// but that would be over the top as we will probably always use hotstuff
			View:           0,
			ParentVoterIDs: nil,
			ParentVoterSig: nil,
			ProposerID:     flow.ZeroID,
			ProposerSig:    nil,
		}

		// apply the custom fields setter of the consensus algorithm
		setter(header)

		// insert the header into the DB
		err = operation.InsertHeader(header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}

		// insert the payload into the DB
		err = procedure.InsertPayload(&payload)(tx)
		if err != nil {
			return fmt.Errorf("could not insert payload: %w", err)
		}

		// index the payload for the block
		err = procedure.IndexPayload(header, &payload)(tx)
		if err != nil {
			return fmt.Errorf("could not index payload: %w", err)
		}

		// index the state commitment for this block
		// TODO this is also done by Mutator.Extend, perhaps refactor that into a procedure
		err = operation.IndexSealIDByBlock(header.ID(), seal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index commit: %w", err)
		}

		return nil
	})

	return header, err
}
