package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type UnincorporatedBlocks struct{}

func (u *UnincorporatedBlocks) Store(blockProposal *types.BlockProposal) {
	panic("TODO")
}

func (u *UnincorporatedBlocks) ExtractByView(view uint64) *types.BlockProposal {
	panic("TODO")
}

func (u *UnincorporatedBlocks) PruneByView(view uint64) {
	panic("TODO")
}
