package follower

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Finalizer implements the finalization logic for followers of consensus. This
// updates protocol state with the new finalized head, but does none of the
// clean-up done by the consensus finalizer.
type Finalizer struct {
	db    *badger.DB
	state protocol.State
}

// NewFinalizer returns a new finalizer for consensus followers.
func NewFinalizer(db *badger.DB, state protocol.State) *Finalizer {
	return &Finalizer{
		db:    db,
		state: state,
	}
}

// MakeFinal finalizes the block with the given ID by updating the protocol state.
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
	}

	return nil
}
