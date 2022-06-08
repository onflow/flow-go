package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Voter produces votes for the given block according to voting rules.
type Voter interface {
	// ProduceVoteIfVotable takes a block and current view, and decides whether to vote for the block.
	// Returns:
	//  * (vote, nil): On the _first_ block for the current view that is safe to vote for.
	//    Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
	//  * (nil, model.NoVoteError): If the voter decides that it does not want to vote for the given block.
	//    This is a sentinel error and _expected_ during normal operation.
	// All other errors are unexpected and potential symptoms of uncovered edge cases or corrupted internal state (fatal).
	ProduceVoteIfVotable(block *model.Block, curView uint64) (*model.Vote, error)
}
