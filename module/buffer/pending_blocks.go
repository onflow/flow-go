package buffer

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// PendingBlocks is a mempool for holding blocks. Furthermore, given a block ID, we can
// query all children that are currently stored in the mempool. The mempool's backend
// is intended to work generically for consensus blocks as well as cluster blocks.
type PendingBlocks struct {
	backend *backend[*flow.Proposal]
}

var _ module.PendingBlockBuffer = (*PendingBlocks)(nil)

func NewPendingBlocks() *PendingBlocks {
	b := &PendingBlocks{backend: newBackend[*flow.Proposal]()}
	return b
}

func (b *PendingBlocks) Add(block flow.Slashable[*flow.Proposal]) bool {
	return b.backend.add(block)
}

func (b *PendingBlocks) ByID(blockID flow.Identifier) (flow.Slashable[*flow.Proposal], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.Slashable[*flow.Proposal]{}, false
	}
	return item.block, true
}

func (b *PendingBlocks) ByParentID(parentID flow.Identifier) ([]flow.Slashable[*flow.Proposal], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	proposals := make([]flow.Slashable[*flow.Proposal], 0, len(items))
	for _, item := range items {
		proposals = append(proposals, item.block)
	}
	return proposals, true
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
