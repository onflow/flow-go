// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Finalizer is a simple wrapper around our temporary state to clean up after a
// block has been fully finalized to the persistent protocol state.
type Finalizer struct {
	db         *badger.DB
	state      protocol.State
	guarantees mempool.Guarantees
	seals      mempool.Seals
}

// NewFinalizer creates a new finalizer for the temporary state.
func NewFinalizer(db *badger.DB, state protocol.State, guarantees mempool.Guarantees, seals mempool.Seals) *Finalizer {
	f := &Finalizer{
		db:         db,
		state:      state,
		guarantees: guarantees,
		seals:      seals,
	}
	return f
}

// MakeFinal will finalize the block with the given ID and clean up the memory
// pools after it.
//
// This assumes that transactions are added to persistent state when they are
// included in a block proposal. Between entering the non-finalized chain state
// and being finalized, entities should be present in both the volatile memory
// pools and persistent storage.
func (f *Finalizer) MakeFinal(blockID flow.Identifier) error {

	// define cleanup function
	cleanup := func(header *flow.Header) error {

		// retrieve IDs of entities to remove from memory pools
		var guaranteeIDs []flow.Identifier
		var sealIDs []flow.Identifier
		err := f.db.View(func(tx *badger.Txn) error {
			err := operation.LookupGuaranteePayload(header.Height, header.ID(), header.ParentID, &guaranteeIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not lookup guarantees: %w", err)
			}
			err = operation.LookupSealPayload(header.Height, header.ID(), header.ParentID, &sealIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not lookup seals: %w", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("could not look up entities: %w", err)
		}

		// clean up the memory pools
		for _, guaranteeID := range guaranteeIDs {
			_ = f.guarantees.Rem(guaranteeID)
		}
		for _, sealID := range sealIDs {
			_ = f.guarantees.Rem(sealID)
		}

		return nil
	}

	return f.state.Mutate().Finalize(blockID, cleanup)
}
