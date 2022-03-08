package buffer

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

type PendingClusterBlocks struct {
	backend *backend
}

func NewPendingClusterBlocks() *PendingClusterBlocks {
	b := &PendingClusterBlocks{backend: newBackend()}
	return b
}

func (b *PendingClusterBlocks) Add(originID flow.Identifier, proposal *messages.ClusterBlockProposal) bool {
	return b.backend.add(originID, proposal.Header, proposal.Payload)
}

func (b *PendingClusterBlocks) ByID(blockID flow.Identifier) (*cluster.PendingBlock, bool) {
	item, ok := b.backend.byID(blockID)
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

// PruneByView prunes any pending cluster blocks with views less or equal to the given view.
func (b *PendingClusterBlocks) PruneByView(view uint64) {
	b.backend.pruneByView(view)
}

func (b *PendingClusterBlocks) Size() uint {
	return b.backend.size()
}
