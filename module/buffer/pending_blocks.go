package buffer

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type PendingBlocks struct {
	backend *backend
}

func NewPendingBlocks() *PendingBlocks {
	b := &PendingBlocks{backend: newBackend()}
	return b
}

func (b *PendingBlocks) Add(block *flow.PendingBlock) bool {
	return b.backend.add(block.OriginID, block.Header, block.Payload)
}

func (b *PendingBlocks) ByID(blockID flow.Identifier) (*flow.PendingBlock, bool) {
	item, ok := b.backend.ByID(blockID)
	if !ok {
		return nil, false
	}

	block := &flow.PendingBlock{
		OriginID: item.originID,
		Header:   item.header,
		Payload:  item.payload.(*flow.Payload),
	}

	return block, true
}

func (b *PendingBlocks) ByParentID(parentID flow.Identifier) ([]*flow.PendingBlock, bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	blocks := make([]*flow.PendingBlock, 0, len(items))
	for _, item := range items {
		block := &flow.PendingBlock{
			OriginID: item.originID,
			Header:   item.header,
			Payload:  item.payload.(*flow.Payload),
		}
		blocks = append(blocks, block)
	}

	return blocks, true
}

func (b *PendingBlocks) DropForParent(parentID flow.Identifier) {
	b.backend.dropForParent(parentID)
}
