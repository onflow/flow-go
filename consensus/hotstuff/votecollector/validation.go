package votecollector

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// VoteComplianceCheck performs general checks against cached block proposal
// Returns nil on success and sentinel model.InvalidVoteError in case vote is invalid
// TODO: this function can be reused vote vote validator by validator/validator.go
func VoteComplianceCheck(vote *model.Vote, block *model.Block) error {
	// block hash must match
	if vote.BlockID != block.BlockID {
		// Sanity check! Failing indicates a bug in the higher-level logic
		return model.NewInvalidVoteErrorf(vote, "wrong block ID. expected (%s), got (%d)", block.BlockID, vote.BlockID)
	}
	// view must match with the block's view
	if vote.View != block.View {
		return model.NewInvalidVoteErrorf(vote, "vote's view %d is inconsistent with referenced block (view %d)", vote.View, block.View)
	}

	return nil
}
