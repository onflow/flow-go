package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Voter struct {
	viewState     ViewState
	lastVotedView uint64
}

// ShouldVoteForCurProposal will make decision on whether it will vote for the given proposal, the returned Vote object
// and boolean indicates whether to vote or not.
// In order to ensure that only safe node will be voted, Voter will ask Forks whether a vote is a safe node or not.
// The curView is taken as input to ensure Voter will only vote for proposals at current view.
// Calling this method the second time for the same inputs will return the same vote as returned for the first time.
func (v *Voter) ShouldVoteForCurProposal(b *types.BlockProposal, curView uint64) (*types.Vote, bool) {
	panic("TODO")
}
