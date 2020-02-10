package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Finalizer represents a wrapper around the payload verification layer that
// handles updating that layer's state when a new block is finalized.
type Finalizer interface {

	// Finalize indicates that the block with the given ID has been finalized.
	// At this point, temporary state in the payload verification layer such as
	// mempools can be cleaned up, and any invalidated forks can be pruned from
	// the chain state.
	Finalize(blockID flow.Identifier)
}
