package module

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// PendingBlockBuffer defines an interface for a cache of pending blocks that
// cannot yet be processed because they do not connect to the rest of the chain
// state. They are indexed by parent ID to enable processing all of a parent's
// children once the parent is received.
// Safe for concurrent use.
type PendingBlockBuffer interface {
	Add(block flow.Slashable[*flow.Block]) bool

	ByID(blockID flow.Identifier) (flow.Slashable[*flow.Block], bool)

	ByParentID(parentID flow.Identifier) ([]flow.Slashable[*flow.Block], bool)

	DropForParent(parentID flow.Identifier)

	// PruneByView prunes any pending blocks with views less or equal to the given view.
	PruneByView(view uint64)

	Size() uint
}

// PendingClusterBlockBuffer is the same thing as PendingBlockBuffer, but for
// collection node cluster consensus.
// Safe for concurrent use.
type PendingClusterBlockBuffer interface {
	Add(block flow.Slashable[*cluster.Block]) bool

	ByID(blockID flow.Identifier) (flow.Slashable[*cluster.Block], bool)

	ByParentID(parentID flow.Identifier) ([]flow.Slashable[*cluster.Block], bool)

	DropForParent(parentID flow.Identifier)

	// PruneByView prunes any pending cluster blocks with views less or equal to the given view.
	PruneByView(view uint64)

	Size() uint
}
