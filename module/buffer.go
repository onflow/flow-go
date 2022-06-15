package module

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// PendingBlockBuffer defines an interface for a cache of pending blocks that
// cannot yet be processed because they do not connect to the rest of the chain
// state. They are indexed by parent ID to enable processing all of a parent's
// children once the parent is received.
type PendingBlockBuffer interface {
	Add(originID flow.Identifier, proposal *messages.BlockProposal) bool

	ByID(blockID flow.Identifier) (*flow.PendingBlock, bool)

	ByParentID(parentID flow.Identifier) ([]*flow.PendingBlock, bool)

	DropForParent(parentID flow.Identifier)

	// PruneByView prunes any pending blocks with views less or equal to the given view.
	PruneByView(view uint64)

	Size() uint
}

// PendingClusterBlockBuffer is the same thing as PendingBlockBuffer, but for
// collection node cluster consensus.
type PendingClusterBlockBuffer interface {
	Add(originID flow.Identifier, proposal *messages.ClusterBlockProposal) bool

	ByID(blockID flow.Identifier) (*cluster.PendingBlock, bool)

	ByParentID(parentID flow.Identifier) ([]*cluster.PendingBlock, bool)

	DropForParent(parentID flow.Identifier)

	// PruneByView prunes any pending cluster blocks with views less or equal to the given view.
	PruneByView(view uint64)

	Size() uint
}
