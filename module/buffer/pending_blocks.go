package buffer

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type PendingBlocks struct {
	backend *backend
}

var _ module.PendingBlockBuffer = (*PendingBlocks)(nil)

func NewPendingBlocks() *PendingBlocks {
	b := &PendingBlocks{backend: newBackend()}
	return b
}

func (b *PendingBlocks) Add(proposal flow.Slashable[flow.Block]) bool {
	block := proposal.Message
	return b.backend.add(proposal.OriginID, block.Header, block.Payload)
}

func (b *PendingBlocks) ByID(blockID flow.Identifier) (flow.Slashable[flow.Block], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.NoSlashable[flow.Block](), false
	}

	block := flow.Slashable[flow.Block]{
		OriginID: item.originID,
		Message: &flow.Block{
			Header:  item.header,
			Payload: item.payload.(*flow.Payload),
		},
	}

	return block, true
}

func (b *PendingBlocks) ByParentID(parentID flow.Identifier) ([]flow.Slashable[flow.Block], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	blocks := make([]flow.Slashable[flow.Block], 0, len(items))
	for _, item := range items {
		block := flow.Slashable[flow.Block]{
			OriginID: item.originID,
			Message: &flow.Block{
				Header:  item.header,
				Payload: item.payload.(*flow.Payload),
			},
		}
		blocks = append(blocks, block)
	}

	return blocks, true
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
