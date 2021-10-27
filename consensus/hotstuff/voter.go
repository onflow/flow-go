package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Voter produces votes for the given block according to voting rules.
type Voter interface {
	// ProduceVoteIfVotable takes a block and current view, and decides whether to vote for the block.
	// If it decides to vote, it returns (&vote, nil). The call will call into VoteAggregator's GetVoteCreator method to get a
	// `createVote` function and create a vote with it,
	// If it decides not to vote, it returns (nil, NoVoteError)
	ProduceVoteIfVotable(block *model.Block, curView uint64) (*model.Vote, error)
}
