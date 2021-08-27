package votecollector

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// VoteValidator contains common logic for validating votes against block proposal.
type VoteValidator struct {
	block *model.Block
}

// Validate performs general checks against cached block proposal
// Returns nil on success and sentinel model.InvalidVoteError in case vote is invalid
func (v *VoteValidator) Validate(vote *model.Vote) error {
	// block hash must match
	if vote.BlockID != v.block.BlockID {
		// Sanity check! Failing indicates a bug in the higher-level logic
		return model.NewInvalidVoteErrorf(vote, "wrong block ID. expected (%s), got (%d)", v.block.BlockID, vote.BlockID)
	}
	// view must match with the block's view
	if vote.View != v.block.View {
		return model.NewInvalidVoteErrorf(vote, "vote's view %d is inconsistent with referenced block (view %d)", vote.View, v.block.View)
	}

	return nil
}
