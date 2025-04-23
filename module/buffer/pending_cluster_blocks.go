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

func (b *PendingClusterBlocks) Add(block flow.Slashable[*cluster.BlockProposal]) bool {
	return b.backend.add(flow.Slashable[*flow.ProposalHeader]{
		OriginID: flow.Identifier{},
		Message:  &flow.ProposalHeader{Header: block.Message.Block.ToHeader(), ProposerSigData: block.Message.ProposerSigData},
	}, block.Message.Block.Payload)
}

func (b *PendingClusterBlocks) ByID(blockID flow.Identifier) (flow.Slashable[*cluster.BlockProposal], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.Slashable[*cluster.BlockProposal]{}, false
	}

	block := flow.Slashable[*cluster.BlockProposal]{
		OriginID: item.header.OriginID,
		Message: &cluster.BlockProposal{
			Block: cluster.NewBlock(
				item.header.Message.Header.HeaderBody,
				*item.payload.(*cluster.Payload),
			),
			ProposerSigData: item.header.Message.ProposerSigData,
		},
	}

	return block, true
}

func (b *PendingClusterBlocks) ByParentID(parentID flow.Identifier) ([]flow.Slashable[*cluster.BlockProposal], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	blocks := make([]flow.Slashable[*cluster.BlockProposal], 0, len(items))
	for _, item := range items {
		block := flow.Slashable[*cluster.BlockProposal]{
			OriginID: item.header.OriginID,
			Message: &cluster.BlockProposal{
				Block: cluster.NewBlock(
					item.header.Message.Header.HeaderBody,
					*item.payload.(*cluster.Payload),
				),
				ProposerSigData: item.header.Message.ProposerSigData,
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
