package module

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// GenericPendingBlockBuffer implements a mempool of pending blocks that cannot yet be processed
// because they do not connect to the rest of the chain state.
// They are indexed by parent ID to enable processing all of a parent's children once the parent is received.
// They are also indexed by view to support pruning.
//
// Safe for concurrent use.
type GenericPendingBlockBuffer[T flow.HashablePayload] interface {
	// Add adds the input block to the block buffer.
	// If the block already exists, or is below the finalized view, this is a no-op.
	// Errors returns:
	//   - mempool.BeyondActiveRangeError if block.View > finalizedView + activeViewRangeSize (when activeViewRangeSize > 0)
	Add(block flow.Slashable[*flow.GenericProposal[T]]) error

	// ByID returns the block with the given ID, if it exists.
	// Otherwise returns (nil, false)
	ByID(blockID flow.Identifier) (flow.Slashable[*flow.GenericProposal[T]], bool)

	// ByParentID returns all direct children of the given block.
	// If no children with the given parent exist, returns (nil, false)
	ByParentID(parentID flow.Identifier) ([]flow.Slashable[*flow.GenericProposal[T]], bool)

	// PruneByView prunes all pending blocks with views less or equal to the given view.
	// Errors returns:
	//   - mempool.BelowPrunedThresholdError if input level is below the lowest retained view (finalized view)
	PruneByView(view uint64) error

	// Size returns the number of blocks in the buffer.
	Size() uint
}

// PendingBlockBuffer is the block buffer for consensus proposals.
type PendingBlockBuffer GenericPendingBlockBuffer[flow.Payload]

// PendingClusterBlockBuffer is the block buffer for cluster proposals.
type PendingClusterBlockBuffer GenericPendingBlockBuffer[cluster.Payload]
