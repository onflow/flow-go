package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Voter struct {
	viewState ViewState
}

func (v *Voter) ConsiderVoting(block *types.BlockProposal, curView uint64) *types.Vote {
	panic("TODO")
}
