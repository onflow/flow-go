package module

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// BufferedProposal generically represents either a [cluster.Proposal] or [flow.Proposal].
type BufferedProposal interface {
	*cluster.Proposal | *flow.Proposal
	ProposalHeader() *flow.ProposalHeader
}

// GenericPendingBlockBuffer implements a mempool of pending blocks that cannot yet be processed
// because they do not connect to the rest of the chain state.
// They are indexed by parent ID to enable processing all of a parent's children once the parent is received.
// They are also indexed by view to support pruning.
// The size of this mempool is partly limited by the enforcement of an allowed view range, however
// a strong size limit also requires that stored proposals are validated to ensure we store only
// one proposal per view. Higher-level logic is responsible for validating proposals prior to storing here.
//
// Safe for concurrent use.
type GenericPendingBlockBuffer[T BufferedProposal] interface {
	// Add adds the input block to the block buffer.
	// If the block already exists, or is below the finalized view, this is a no-op.
	Add(block flow.Slashable[T])

	// ByID returns the block with the given ID, if it exists.
	// Otherwise returns (nil, false)
	ByID(blockID flow.Identifier) (flow.Slashable[T], bool)

	// ByParentID returns all direct children of the given block.
	// If no children with the given parent exist, returns (nil, false)
	ByParentID(parentID flow.Identifier) ([]flow.Slashable[T], bool)

	// PruneByView prunes all pending blocks with views less or equal to the given view.
	// Errors returns:
	//   - mempool.BelowPrunedThresholdError if input level is below the lowest retained view (finalized view)
	PruneByView(view uint64) error

	// Size returns the number of blocks in the buffer.
	Size() uint
}

// PendingBlockBuffer is the block buffer for consensus proposals.
type PendingBlockBuffer GenericPendingBlockBuffer[*flow.Proposal]

// PendingClusterBlockBuffer is the block buffer for cluster proposals.
type PendingClusterBlockBuffer GenericPendingBlockBuffer[*cluster.Proposal]
