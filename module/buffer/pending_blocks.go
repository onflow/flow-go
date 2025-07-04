package buffer

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// PendingBlocks is a mempool for holding blocks. Furthermore, given a block ID, we can
// query all children that are currently stored in the mempool. The mempool's backend
// is intended to work generically for consensus blocks as well as cluster blocks.
// TODO: this mempool was implemented prior to generics being available in Go. Hence, the
// backend abstracts the payload as an interface{}. This should be updated to use generics.
type PendingBlocks struct {
	backend *backend
}

var _ module.PendingBlockBuffer = (*PendingBlocks)(nil)

func NewPendingBlocks() *PendingBlocks {
	b := &PendingBlocks{backend: newBackend()}
	return b
}

func (b *PendingBlocks) Add(block flow.Slashable[*flow.BlockProposal]) bool {
	return b.backend.add(flow.Slashable[*flow.ProposalHeader]{
		OriginID: block.OriginID,
		Message:  block.Message.ProposalHeader(),
	}, block.Message.Block.Payload)
}

func (b *PendingBlocks) ByID(blockID flow.Identifier) (flow.Slashable[*flow.BlockProposal], bool) {
	item, ok := b.backend.byID(blockID)
	if !ok {
		return flow.Slashable[*flow.BlockProposal]{}, false
	}

	block, err := flow.NewBlock(
		flow.UntrustedBlock{
			Header:  item.header.Message.Header.HeaderBody,
			Payload: item.payload.(flow.Payload),
		},
	)
	if err != nil {
		return flow.Slashable[*flow.BlockProposal]{}, false
	}
	proposal := flow.Slashable[*flow.BlockProposal]{
		OriginID: item.header.OriginID,
		Message: &flow.BlockProposal{
			Block:           *block,
			ProposerSigData: item.header.Message.ProposerSigData,
		},
	}

	return proposal, true
}

func (b *PendingBlocks) ByParentID(parentID flow.Identifier) ([]flow.Slashable[*flow.BlockProposal], bool) {
	items, ok := b.backend.byParentID(parentID)
	if !ok {
		return nil, false
	}

	proposals := make([]flow.Slashable[*flow.BlockProposal], 0, len(items))
	for _, item := range items {
		block, err := flow.NewBlock(
			flow.UntrustedBlock{
				Header:  item.header.Message.Header.HeaderBody,
				Payload: item.payload.(flow.Payload),
			},
		)
		if err != nil {
			return nil, false
		}

		proposal := flow.Slashable[*flow.BlockProposal]{
			OriginID: item.header.OriginID,
			Message: &flow.BlockProposal{
				Block:           *block,
				ProposerSigData: item.header.Message.ProposerSigData,
			},
		}
		proposals = append(proposals, proposal)
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
