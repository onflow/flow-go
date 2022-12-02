package buffer

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type PendingBlocks[P flow.AnyPayload] struct {
	// TODO move backend into PendingBlocks
	backend *backend[P]
}

var _ module.PendingBlockBuffer = (*PendingBlocks)(nil)

func NewPendingBlocks[P flow.AnyPayload]() *PendingBlocks[P] {
	b := &PendingBlocks[P]{backend: newBackend[P]()}
	return b
}

func (b *PendingBlocks[P]) Add(block flow.Slashable[flow.AnyBlock[P]]) bool {
	return b.backend.add(block)
}

func (b *PendingBlocks[P]) ByID(blockID flow.Identifier) (flow.Slashable[flow.AnyBlock[P]], bool) {
	block, ok := b.backend.byID(blockID)
	return block, ok
}

func (b *PendingBlocks[P]) ByParentID(parentID flow.Identifier) ([]flow.Slashable[flow.AnyBlock[P]], bool) {
	items, ok := b.backend.byParentID(parentID)
	return items, ok
	if !ok {
		return nil, false
	}
}

func (b *PendingBlocks) DropForParent(parentID flow.Identifier) {
	b.backend.dropForParent(parentID)
}

// PruneByView prunes any pending blocks with views less or equal to the given view.
func (b *PendingBlocks) PruneByView(view uint64) {
	b.backend.pruneByView(view)
}

func (b *PendingBlocks) Size() uint {
	return b.backend.size()
}
