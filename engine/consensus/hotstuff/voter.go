package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Voter struct {
	viewState ViewState
}

// TODO: api unclear. requires documentation to specify expected behaviour
func (v *Voter) ShouldVoteForNewProposal(b *types.BlockProposal, curView uint64) (myVote *types.Vote, voteCollectorIdx uint32) {
	panic("TODO")
}
