package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Pruner represents a wrapper around the chain state that handles removing
// invalidated forks.
type Pruner interface {

	// PruneAfter prunes any invalidated forks when a new block is finalized.
	// A fork is invalidated when it becomes impossible for it to ever be
	// finalized.
	PruneAfter(blockID flow.Identifier)
}
