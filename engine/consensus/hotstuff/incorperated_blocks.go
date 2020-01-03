package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type IncorperatedBlocks struct{}

func (ib *IncorperatedBlocks) Store(blockProposal *types.BlockProposal) {
	panic("TODO")
}

func (ib *IncorperatedBlocks) ExtractByView(view uint64) *types.BlockProposal {
	panic("TODO")
}

func (ib *IncorperatedBlocks) PruneByView(view uint64) {
	panic("TODO")
}
