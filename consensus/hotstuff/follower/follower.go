package follower

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// FollowerLogic runs in non-consensus nodes. It informs other components within the node
// about finalization of blocks. The consensus Follower consumes all block proposals
// broadcasts by the consensus node, verifies the block header and locally evaluates
// the finalization rules.
//
// CAUTION: Follower is NOT CONCURRENCY safe
type FollowerLogic struct {
	log               zerolog.Logger
	validator         hotstuff.Validator
	finalizationLogic hotstuff.Forks
}

// New creates a new FollowerLogic instance
func New(
	log zerolog.Logger,
	validator hotstuff.Validator,
	finalizationLogic hotstuff.Forks,
) (*FollowerLogic, error) {
	return &FollowerLogic{
		log:               log.With().Str("hotstuff", "follower").Logger(),
		validator:         validator,
		finalizationLogic: finalizationLogic,
	}, nil
}

// FinalizedBlock returns the latest finalized block
func (f *FollowerLogic) FinalizedBlock() *model.Block {
	return f.finalizationLogic.FinalizedBlock()
}

// AddBlock processes the given block proposal
func (f *FollowerLogic) AddBlock(blockProposal *model.Proposal) error {
	err := f.finalizationLogic.AddProposal(blockProposal)
	if err != nil {
		return fmt.Errorf("finalization logic cannot process block proposal %x: %w", blockProposal.Block.BlockID, err)
	}

	return nil
}
