package buffer

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

type PendingClusterBlocks struct {
	backend *backend
}

func NewPendingClusterBlocks() *PendingClusterBlocks {
	b := &PendingClusterBlocks{backend: newBackend()}
	return b
}

func (b *PendingClusterBlocks) Add(block flow.Slashable[*cluster.Block]) bool {
	return b.backend.add(flow.Slashable[*flow.Header]{
		OriginID: flow.Identifier{},
		Message:  block.Message.Header,
	}, block.Message.Payload)
}

func (b *PendingClusterBlocks) ByID(blockID flow.Identifier) (flow.Slashable[*cluster.Block], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.Slashable[*cluster.Block]{}, false
	}

	block := flow.Slashable[*cluster.Block]{
		OriginID: item.header.OriginID,
		Message: &cluster.Block{
			Header:  item.header.Message,
			Payload: item.payload.(*cluster.Payload),
		},
	}

	return block, true
}

func (b *PendingClusterBlocks) ByParentID(parentID flow.Identifier) ([]flow.Slashable[*cluster.Block], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	blocks := make([]flow.Slashable[*cluster.Block], 0, len(items))
	for _, item := range items {
		block := flow.Slashable[*cluster.Block]{
			OriginID: item.header.OriginID,
			Message: &cluster.Block{
				Header:  item.header.Message,
				Payload: item.payload.(*cluster.Payload),
			},
		}
		blocks = append(blocks, block)
	}

	return blocks, true
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
