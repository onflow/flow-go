package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// Voter produces votes for the given block
type Voter interface {

	// ProduceVoteIfVotable will produce a vote for the given block if voting on
	// the given block is a valid action.
	ProduceVoteIfVotable(block *model.Block, curView uint64) (*model.Vote, error)
}
