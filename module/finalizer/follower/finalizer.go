package follower

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Finalizer implements the finalization logic for followers of consensus. This
// updates protocol state with the new finalized head, but does none of the
// clean-up done by the consensus finalizer.
type Finalizer struct {
	db *badger.DB
}

// NewFinalizer returns a new finalizer for consensus followers.
func NewFinalizer(db *badger.DB) *Finalizer {
	return &Finalizer{db: db}
}

// MakeFinal finalizes the block with the given ID by updating the protocol state.
func (f *Finalizer) MakeFinal(blockID flow.Identifier) error {
	return f.db.Update(func(tx *badger.Txn) error {

		var toFinalize []*flow.Header
		err := procedure.RetrieveUnfinalizedAncestors(blockID, &toFinalize)(tx)
		if err != nil {
			return fmt.Errorf("could not get blocks to finalize: %w", err)
		}

		// now we can step backwards in order to go from oldest to youngest; for
		// each header, we reconstruct the block and then apply the related
		// changes to the protocol state
		for i := len(toFinalize) - 1; i >= 0; i-- {

			step := toFinalize[i]

			// finalize the block in the protocol state
			err := procedure.FinalizeBlock(step.ID())(tx)
			if err != nil {
				return fmt.Errorf("could not finalize block (%x): %w", blockID, err)
			}
		}

		return nil
	})
}

func (f *Finalizer) MakeTentative(blockID flow.Identifier, parentID flow.Identifier) error {
	return f.db.Update(func(tx *badger.Txn) error {
		err := procedure.IndexChildByBlockID(parentID, blockID)(tx)
		if err != nil {
			return fmt.Errorf("cannot index child by blockID: %w", err)
		}

		return nil
	})
}
