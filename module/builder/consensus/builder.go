// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Builder is the builder for block payloads. Upon providing a payload hash, it
// also memorizes which entities were included into the payload.
type Builder struct {
	db         *badger.DB
	state      protocol.State
	guarantees mempool.Guarantees
	seals      mempool.Seals
}

// NewBuilder creates a new block builder.
func NewBuilder(db *badger.DB, state protocol.State, guarantees mempool.Guarantees, seals mempool.Seals) *Builder {
	b := &Builder{
		db:         db,
		state:      state,
		guarantees: guarantees,
		seals:      seals,
	}
	return b
}

// BuildOn creates a new block payload on top of the provided parent.
func (b *Builder) BuildOn(parentID flow.Identifier, build module.BuildFunc) (*flow.Header, error) {
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

		// for each unfinalized ancestor of the payload we are building, we retrieve
		// a list of all pending IDs for guarantees and seals; we can use them to
		// exclude entities from being included in two block on the same fork
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
			if ancestor.Number <= boundary {
				break
			}

			// look up the ancestor's guarantees
			var guaranteeIDs []flow.Identifier
			err := operation.LookupGuarantees(ancestor.PayloadHash, &guaranteeIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not look up ancestor guarantees (%x): %w", ancestor.PayloadHash, err)
			}

			// look up the ancestor's seals
			var sealIDs []flow.Identifier
			err = operation.LookupSeals(ancestor.PayloadHash, &sealIDs)(tx)
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
		var guarantees []*flow.CollectionGuarantee
		for _, guarantee := range b.guarantees.All() {
			_, ok := guaranteeLookup[guarantee.ID()]
			if ok {
				continue
			}
			guarantees = append(guarantees, guarantee)
		}

		// get the finalized state commitment at the parent
		var commit flow.StateCommitment
		err = operation.RetrieveCommit(parentID, &commit)(tx)
		if err != nil {
			return fmt.Errorf("could not get parent state commit: %w", err)
		}

		// we then keep adding seals that follow this state commit from the pool
		var seals []*flow.Seal
		for {

			// get a seal that extends the last known state commitment
			seal, err := b.seals.ByPreviousState(commit)
			if errors.Is(err, mempool.ErrEntityNotFound) {
				break
			}
			if err != nil {
				return fmt.Errorf("could not get extending seal (%x): %w", commit, err)
			}

			// add the seal to our list and forward to the known last valid state
			seals = append(seals, seal)
			commit = seal.FinalState
		}

		// STEP THREE: we now have the guarantees and seals we can validly
		// include in the payload built on top of the given block; but we still
		// need to memorize them before returning, so we can rebuild it later

		// build the payload so we can get the hash
		payload := flow.Payload{
			Identities: nil,
			Guarantees: guarantees,
			Seals:      seals,
		}
		payloadHash := payload.Hash()

		// index the guarantees for the payload
		for _, guarantee := range guarantees {
			err = operation.IndexGuarantee(payloadHash, guarantee.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not index guarantee (%x): %w", guarantee.ID(), err)
			}
		}

		// index the seals for the payload
		for _, seal := range seals {
			err = operation.IndexSeal(payloadHash, seal.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not index seal (%x): %w", seal.ID(), err)
			}
		}

		// generate the block using the hotstuff function
		header, err = build(payloadHash)
		if err != nil {
			return fmt.Errorf("could not build block: %w", err)
		}

		// convert into flow header and store
		err = operation.InsertHeader(header)(tx)
		if err != nil {
			return fmt.Errorf("could not insert header: %w", err)
		}

		return nil
	})

	return header, err
}
