package module

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// GenericPendingBlockBuffer defines an interface for a cache of pending blocks that
// cannot yet be processed because they do not connect to the rest of the chain
// state. They are indexed by parent ID to enable processing all of a parent's
// children once the parent is received.
type GenericPendingBlockBuffer[P flow.GenericPayload] interface {
	Add(originID flow.Identifier, block *flow.GenericBlock[P]) bool

	ByID(blockID flow.Identifier) (flow.Slashable[flow.GenericBlock[P]], bool)

	ByParentID(parentID flow.Identifier) ([]flow.Slashable[flow.GenericBlock[P]], bool)

	DropForParent(parentID flow.Identifier)

	// PruneByView prunes any pending blocks with views less or equal to the given view.
	PruneByView(view uint64)

	Size() uint
}

type PendingBlockBuffer = GenericPendingBlockBuffer[*flow.Payload]
type PendingClusterBlockBuffer = GenericPendingBlockBuffer[*cluster.Payload]
