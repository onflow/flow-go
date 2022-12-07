package buffer

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type GenericPendingBlockBuffer[P flow.GenericPayload] struct {
	// TODO move backend into GenericPendingBlockBuffer
	backend *backend[P]
}

var _ module.PendingBlockBuffer = (*GenericPendingBlockBuffer[*flow.Payload])(nil)
var _ module.PendingClusterBlockBuffer = (*GenericPendingBlockBuffer[*cluster.Payload])(nil)

func NewPendingBlocks() *GenericPendingBlockBuffer[*flow.Payload] {
	b := &GenericPendingBlockBuffer[*flow.Payload]{backend: newBackend[*flow.Payload]()}
	return b
}

func NewPendingClusterBlocks() *GenericPendingBlockBuffer[*cluster.Payload] {
	b := &GenericPendingBlockBuffer[*cluster.Payload]{backend: newBackend[*cluster.Payload]()}
	return b
}

func (b *GenericPendingBlockBuffer[P]) Add(originID flow.Identifier, block *flow.GenericBlock[P]) bool {
	return b.backend.add(originID, block)
}

func (b *GenericPendingBlockBuffer[P]) ByID(blockID flow.Identifier) (flow.Slashable[flow.GenericBlock[P]], bool) {
	block, ok := b.backend.byID(blockID)
	return block, ok
}

func (b *GenericPendingBlockBuffer[P]) ByParentID(parentID flow.Identifier) ([]flow.Slashable[flow.GenericBlock[P]], bool) {
	items, ok := b.backend.byParentID(parentID)
	return items, ok
}

func (b *GenericPendingBlockBuffer[P]) DropForParent(parentID flow.Identifier) {
	b.backend.dropForParent(parentID)
}

// PruneByView prunes any pending blocks with views less or equal to the given view.
func (b *GenericPendingBlockBuffer[P]) PruneByView(view uint64) {
	b.backend.pruneByView(view)
}

func (b *GenericPendingBlockBuffer[P]) Size() uint {
	return b.backend.size()
}
