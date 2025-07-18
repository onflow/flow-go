package buffer

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type PendingClusterBlocks struct {
	backend *backend[*cluster.Proposal]
}

func NewPendingClusterBlocks() *PendingClusterBlocks {
	b := &PendingClusterBlocks{backend: newBackend[*cluster.Proposal]()}
	return b
}

func (b *PendingClusterBlocks) Add(block flow.Slashable[*cluster.Proposal]) bool {
	return b.backend.add(block)
}

func (b *PendingClusterBlocks) ByID(blockID flow.Identifier) (flow.Slashable[*cluster.Proposal], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.Slashable[*cluster.Proposal]{}, false
	}
	return item.block, true
}

func (b *PendingClusterBlocks) ByParentID(parentID flow.Identifier) ([]flow.Slashable[*cluster.Proposal], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	proposals := make([]flow.Slashable[*cluster.Proposal], 0, len(items))
	for _, item := range items {
		proposals = append(proposals, item.block)
	}
	return proposals, true
}

func (b *PendingClusterBlocks) DropForParent(parentID flow.Identifier) {
	b.backend.dropForParent(parentID)
}

// PruneByView prunes any pending cluster blocks with views less or equal to the given view.
func (b *PendingClusterBlocks) PruneByView(view uint64) {
	b.backend.pruneByView(view)
}

func (b *PendingClusterBlocks) Size() uint {
	return b.backend.size()
}
