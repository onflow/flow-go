package votecollector

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

var (
	// VoteForIncompatibleViewError is emitted, if a view-specific component
	// receives a vote for a different view number.
	VoteForIncompatibleViewError = errors.New("vote for incompatible view")

	// VoteForIncompatibleBlockError is emitted, if a block-specific component
	// receives a vote for a different block ID.
	VoteForIncompatibleBlockError = errors.New("vote for incompatible block")

	// DuplicatedVoteErr is emitted, when we receive an _identical_ duplicated
	// vote for the same block from the same block. This error does _not_
	// indicate equivocation.
	DuplicatedVoteErr = errors.New("duplicated vote")
)

/******************************* NoopProcessor *******************************/

// NoopProcessor implements hotstuff.VoteProcessor. It drops all votes.
type NoopProcessor struct {
	status hotstuff.VoteCollectorStatus
}

func NewNoopCollector(status hotstuff.VoteCollectorStatus) *NoopProcessor {
	return &NoopProcessor{status}
}

func (c *NoopProcessor) Process(*model.Vote) error            { return nil }
func (c *NoopProcessor) Status() hotstuff.VoteCollectorStatus { return c.status }

/************************ enforcing vote is for block ************************/

// EnsureVoteForBlock verifies that the vote is for the given block.
// Returns nil on success and sentinel errors:
//  * model.VoteForIncompatibleViewError if the vote is from a different view than block
//  * model.VoteForIncompatibleBlockError if the vote is from the same view as block
//    but for a different blockID
func EnsureVoteForBlock(vote *model.Vote, block *model.Block) error {
	if vote.View != block.View {
		return fmt.Errorf("vote %v has view %d while block's view is %d: %w ", vote.ID(), vote.View, block.View, VoteForIncompatibleViewError)
	}
	if vote.BlockID != block.BlockID {
		return fmt.Errorf("expecting only votes for block %v, but vote %v is for block %v: %w ", block.BlockID, vote.ID(), vote.BlockID, VoteForIncompatibleBlockError)
	}
	return nil
}
