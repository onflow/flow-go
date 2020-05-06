package buffer

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

type PendingClusterBlocks struct {
	backend *backend
}

func NewPendingClusterBlocks() *PendingClusterBlocks {
	b := &PendingClusterBlocks{backend: newBackend()}
	return b
}

func (b *PendingClusterBlocks) Add(block *cluster.PendingBlock) bool {
	return b.backend.add(block.OriginID, block.Header, block.Payload)
}

func (b *PendingClusterBlocks) ByID(blockID flow.Identifier) (*cluster.PendingBlock, bool) {
	item, ok := b.backend.ByID(blockID)
	if !ok {
		return nil, false
	}

	block := &cluster.PendingBlock{
		OriginID: item.originID,
		Header:   item.header,
		Payload:  item.payload.(*cluster.Payload),
	}

	return block, true
}

func (b *PendingClusterBlocks) ByParentID(parentID flow.Identifier) ([]*cluster.PendingBlock, bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	blocks := make([]*cluster.PendingBlock, 0, len(items))
	for _, item := range items {
		block := &cluster.PendingBlock{
			OriginID: item.originID,
			Header:   item.header,
			Payload:  item.payload.(*cluster.Payload),
		}
		blocks = append(blocks, block)
	}

	return blocks, true
}

func (b *PendingClusterBlocks) DropForParent(parentID flow.Identifier) {
	b.backend.dropForParent(parentID)
}

func (b *PendingClusterBlocks) PruneByHeight(height uint64) {
	b.backend.pruneByHeight(height)
}

func (b *PendingClusterBlocks) Size() uint {
	return uint(len(b.backend.byID))
}
