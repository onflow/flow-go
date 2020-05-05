// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Finalizer is a simple wrapper around our temporary state to clean up after a
// block has been fully finalized to the persistent protocol state.
type Finalizer struct {
	db         *badger.DB
	guarantees mempool.Guarantees
	seals      mempool.Seals
}

// NewFinalizer creates a new finalizer for the temporary state.
func NewFinalizer(db *badger.DB, guarantees mempool.Guarantees, seals mempool.Seals) *Finalizer {
	f := &Finalizer{
		db:         db,
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
	return f.db.Update(func(tx *badger.Txn) error {

		var toFinalize []*flow.Header
		err := procedure.RetrieveUnfinalizedAncestors(blockID, &toFinalize)(tx)
		if err != nil {
			return fmt.Errorf("could not get blocks to finalize: %w", err)
		}

		// if there are no blocks to finalize, we may have already finalized
		// this block - exit early
		if len(toFinalize) == 0 {
			return nil
		}

		// now we can step backwards in order to go from oldest to youngest; for
		// each header, we reconstruct the block and then apply the related
		// changes to the protocol state
		for i := len(toFinalize) - 1; i >= 0; i-- {

			// look up the list of guarantee IDs included in the payload
			step := toFinalize[i]
			var guaranteeIDs []flow.Identifier
			err = operation.LookupGuaranteePayload(step.Height, step.ID(), step.ParentID, &guaranteeIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not look up guarantees (block_id=%x): %w", step.ID(), err)
			}

			// look up list of seal IDs included in the payload
			var sealIDs []flow.Identifier
			err = operation.LookupSealPayload(step.Height, step.ID(), step.ParentID, &sealIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not look up seals (block_id=%x): %w", step.ID(), err)
			}

			// remove the guarantees from the memory pool
			for _, guaranteeID := range guaranteeIDs {
				// we don't care if it still exists in the mempool
				// if it doesn't exist, it's probably lost during a restart
				_ = f.guarantees.Rem(guaranteeID)
			}

			// remove all seals from the memory pool
			for _, sealID := range sealIDs {
				// we don't care if it still exists in the mempool
				// if it doesn't exist, it's probably lost during a restart
				_ = f.seals.Rem(sealID)
			}

			// finalize the block in the protocol state
			err := procedure.FinalizeBlock(step.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not finalize block (%x): %w", blockID, err)
			}
		}

		return nil
	})
}
