package voteAggregator

import (
	"github.com/dapperlabs/flow-go/engine/consensus/modules/def"
	"github.com/dapperlabs/flow-go/engine/consensus/modules/defConAct"
)

//VoteAggregator aggregates incomming
type VoteAggregator struct {
}

func (v *VoteAggregator) OnIncorporatedBlock(block *def.Block) {
	panic("Implement me")
}

func (v *VoteAggregator) OnEnteringView(view uint64) {
	panic("Implement me")
}

func (v *VoteAggregator) OnReceivedVote(vote *defConAct.Vote) {
	panic("Implement me")
}
