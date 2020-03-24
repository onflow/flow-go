// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/engine/consensus/provider"
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
	prov       *provider.Engine // to propagate finalized blocks to non-consensus nodes
}

// NewFinalizer creates a new finalizer for the temporary state.
func NewFinalizer(db *badger.DB, guarantees mempool.Guarantees, seals mempool.Seals, prov *provider.Engine) *Finalizer {
	f := &Finalizer{
		db:         db,
		guarantees: guarantees,
		seals:      seals,
		prov:       prov,
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

		// retrieve the block header of the block we want to finalize
		var header flow.Header
		err := operation.RetrieveHeader(blockID, &header)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header: %w", err)
		}

		// retrieve the current boundary of the finalized state
		var boundary uint64
		err = operation.RetrieveBoundary(&boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// retrieve the ID of the last finalized block as marker for stopping
		var headID flow.Identifier
		err = operation.RetrieveNumber(boundary, &headID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// in order to validate the validity of all changes, we need to iterate
		// through the blocks that need to be finalized from oldest to youngest;
		// we thus start at the youngest remember all of the intermediary steps
		// while tracing back until we reach the finalized state
		steps := []*flow.Header{&header}
		parentID := header.ParentID
		for parentID != headID {
			var parent flow.Header
			err = operation.RetrieveHeader(parentID, &parent)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve parent (%x): %w", parentID, err)
			}
			steps = append(steps, &parent)
			parentID = parent.ParentID
		}

		// now we can step backwards in order to go from oldest to youngest; for
		// each header, we reconstruct the block and then apply the related
		// changes to the protocol state
		for i := len(steps) - 1; i >= 0; i-- {

			// look up the list of guarantee IDs included in the payload
			step := steps[i]
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
				ok := f.guarantees.Rem(guaranteeID)
				if !ok {
					return fmt.Errorf("could not remove guarantee (collID: %x)", guaranteeID)
				}
			}

			// remove all seals from the memory pool
			for _, sealID := range sealIDs {
				ok := f.seals.Rem(sealID)
				if !ok {
					return fmt.Errorf("could not remove seal (sealID: %x)", sealID)
				}
			}

			// finalize the block in the protocol state
			err := procedure.FinalizeBlock(step.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not finalize block (%x): %w", blockID, err)
			}
		}

		// retrieve the payload, build the full block, and propagate to non-consensus nodes
		// TODO this is only necessary to replicate existing block propagation behaviour
		// This should soon be replaced with HotStuff follower https://github.com/dapperlabs/flow/issues/894
		{
			// get the payload
			var payload flow.Payload
			err = procedure.RetrievePayload(header.ID(), &payload)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve payload: %w", err)
			}

			// create the block
			block := &flow.Block{
				Header:  header,
				Payload: payload,
			}

			// finally broadcast to non-consensus nodes
			f.prov.SubmitLocal(block)
		}

		return nil
	})
}
