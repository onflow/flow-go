package follower

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
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

	// define cleanup function
	cleanup := func(header *flow.Header) error {
		return nil
	}

	return f.state.Mutate().Finalize(blockID, cleanup)
}
