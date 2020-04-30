// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Finalizer is a simple wrapper around our temporary state to clean up after a
// block has been fully finalized to the persistent protocol state.
type Finalizer struct {
	db         *badger.DB
	pcache     module.PayloadCache
	guarantees mempool.Guarantees
	seals      mempool.Seals
}

// NewFinalizer creates a new finalizer for the temporary state.
func NewFinalizer(db *badger.DB, pcache module.PayloadCache, guarantees mempool.Guarantees, seals mempool.Seals) *Finalizer {
	f := &Finalizer{
		db:         db,
		pcache:     pcache,
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

	// headers we want to finalize
	var headers []*flow.Header

	// execute the finalization checks needed
	err := f.db.View(func(tx *badger.Txn) error {

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

		// check if we are finalizing an invalid block
		if header.Height <= boundary {
			return fmt.Errorf("height below or equal to boundary (height: %d, boundary: %d)", header.Height, boundary)
		}

		// retrieve the finalized block ID
		var finalID flow.Identifier
		err = operation.RetrieveNumber(boundary, &finalID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve head: %w", err)
		}

		// in order to validate the validity of all changes, we need to iterate
		// through the blocks that need to be finalized from oldest to youngest;
		// we thus start at the youngest remember all of the intermediary steps
		// while tracing back until we reach the finalized state
		headers = append(headers, &header)

		// create a copy of header for the loop to not change the header the slice above points to
		ancestorID := header.ParentID
		for ancestorID != finalID {
			var ancestor flow.Header
			err = operation.RetrieveHeader(ancestorID, &ancestor)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve parent (%x): %w", header.ParentID, err)
			}
			headers = append(headers, &ancestor)
			ancestorID = ancestor.ParentID
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("could not check finalization: %w", err)
	}

	// now we can step backwards in order to go from oldest to youngest; for
	// each header, we reconstruct the block and then apply the related
	// changes to the protocol state
	for i := len(headers) - 1; i >= 0; i-- {
		header := headers[i]

		// finalize the block
		err = operation.RetryOnConflict(func() error {
			return f.db.Update(func(tx *badger.Txn) error {

				// insert the number to block mapping
				err = operation.InsertNumber(header.Height, header.ID())(tx)
				if err != nil {
					return fmt.Errorf("could not insert number mapping: %w", err)
				}

				// update the finalized boundary
				err = operation.UpdateBoundary(header.Height)(tx)
				if err != nil {
					return fmt.Errorf("could not update finalized boundary: %w", err)
				}

				return nil
			})
		})
		if err != nil {
			return fmt.Errorf("could not execute finalization (header: %x): %w", header.ID(), err)
		}

		// allow other components to clean up after block
		// retrieve IDs of entities to remove from memory pools
		var guaranteeIDs []flow.Identifier
		var sealIDs []flow.Identifier
		err := f.db.View(func(tx *badger.Txn) error {
			// TODO: can optimize by using mempool
			err := operation.LookupGuaranteePayload(header.Height, header.ID(), header.ParentID, &guaranteeIDs)(tx)
			if err != nil {
				return fmt.Errorf("could not lookup guarantees: %w", err)
			}
			// TODO: can optimize by using mempool
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

		// clean up the payload cache
		f.pcache.AddGuarantees(header.Height, guaranteeIDs)
		f.pcache.AddSeals(header.Height, sealIDs)
	}

	return nil
}
