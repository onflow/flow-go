package buffer

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type GenericPendingBlocks[T module.BufferedProposal] struct {
	backend *backend[T]
}

type PendingClusterBlocks = GenericPendingBlocks[*cluster.Proposal]
type PendingBlocks = GenericPendingBlocks[*flow.Proposal]

func NewPendingClusterBlocks() *PendingClusterBlocks {
	return &PendingClusterBlocks{backend: newBackend[*cluster.Proposal]()}
}

func NewPendingBlocks() *PendingBlocks {
	return &PendingBlocks{backend: newBackend[*flow.Proposal]()}
}

func (b *GenericPendingBlocks[T]) Add(block flow.Slashable[T]) bool {
	return b.backend.add(block)
}

func (b *GenericPendingBlocks[T]) ByID(blockID flow.Identifier) (flow.Slashable[T], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.Slashable[T]{}, false
	}
	return item.block, true
}

func (b *GenericPendingBlocks[T]) ByParentID(parentID flow.Identifier) ([]flow.Slashable[T], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	proposals := make([]flow.Slashable[T], 0, len(items))
	for _, item := range items {
		proposals = append(proposals, item.block)
	}
	return proposals, true
}

func (b *GenericPendingBlocks[T]) DropForParent(parentID flow.Identifier) {
	b.backend.dropForParent(parentID)
}

// PruneByView prunes any pending cluster blocks with views less or equal to the given view.
func (b *GenericPendingBlocks[T]) PruneByView(view uint64) {
	b.backend.pruneByView(view)
}

func (b *GenericPendingBlocks[T]) Size() uint {
	return b.backend.size()
}
