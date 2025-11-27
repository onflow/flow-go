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

// GenericPendingBlockBuffer defines an interface for a cache of pending blocks that
// cannot yet be processed because they do not connect to the rest of the chain
// state. They are indexed by parent ID to enable processing all of a parent's
// children once the parent is received.
// TODO paste doc from impl
// Safe for concurrent use.
type GenericPendingBlockBuffer[T BufferedProposal] interface {
	Add(block flow.Slashable[T])

	ByID(blockID flow.Identifier) (flow.Slashable[T], bool)

	ByParentID(parentID flow.Identifier) ([]flow.Slashable[T], bool)

	DropForParent(parentID flow.Identifier)

	// PruneByView prunes any pending blocks with views less or equal to the given view.
	PruneByView(view uint64)

	Size() uint
}

// PendingBlockBuffer is the block buffer for consensus proposals.
type PendingBlockBuffer = GenericPendingBlockBuffer[*flow.Proposal]

// PendingClusterBlockBuffer is the block buffer for cluster proposals.
type PendingClusterBlockBuffer = GenericPendingBlockBuffer[*cluster.Proposal]
