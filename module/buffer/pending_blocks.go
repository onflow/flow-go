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

func (b *PendingBlocks) Add(block flow.Slashable[*flow.Block]) bool {
	return b.backend.add(flow.Slashable[*flow.Header]{
		OriginID: block.OriginID,
		Message:  block.Message.Header,
	}, block.Message.Payload)
}

func (b *PendingBlocks) ByID(blockID flow.Identifier) (flow.Slashable[*flow.Block], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.Slashable[*flow.Block]{}, false
	}

	block := flow.Slashable[*flow.Block]{
		OriginID: item.header.OriginID,
		Message: &flow.Block{
			Header:  item.header.Message,
			Payload: item.payload.(*flow.Payload),
		},
	}

	return block, true
}

func (b *PendingBlocks) ByParentID(parentID flow.Identifier) ([]flow.Slashable[*flow.Block], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	blocks := make([]flow.Slashable[*flow.Block], 0, len(items))
	for _, item := range items {
		block := flow.Slashable[*flow.Block]{
			OriginID: item.header.OriginID,
			Message: &flow.Block{
				Header:  item.header.Message,
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
