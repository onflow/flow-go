package voteAggregator

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/defConAct"
)

// VoteAggregator aggregates incoming votes and generates a QC if possible.
// VoteAggregator consumes the `IncorporatedBlock` event for the following reasons:
// - In order to generate a QC, VoteAggregator needs to access the nodes' stakes
// - Staking information is part of the protocol state and requires
//   that all ancestors of a block are known
// - This means that the block which is voted for needs to be in the main chain
// - as soon as a block is added to the main chain,
//   VoteAggregator is notified and can track this information
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
