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
// This method will only ever _once_ return a `non-nil, true` vote: the very first time it encounters a safe block of the current view to vote for. Subsequently, voter does _not_ vote for any other block with the same (or lower) view. (including repeated calls with the initial block we voted for also return `nil,false`). 
func (v *Voter) ShouldVoteForCurProposal(b *types.BlockProposal, curView uint64) (*types.Vote, bool) {
	panic("TODO")
}
