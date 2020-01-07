package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type UnincorperatedBlocks struct{}

func (u *UnincorperatedBlocks) Store(blockProposal *types.BlockProposal) {
	panic("TODO")
}

func (u *UnincorperatedBlocks) ExtractByView(view uint64) *types.BlockProposal {
	panic("TODO")
}

func (u *UnincorperatedBlocks) PruneByView(view uint64) {
	panic("TODO")
}
