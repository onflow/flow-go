package follower

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/logging"
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
	// validate the block. skip if the proposal is invalid
	err := f.validator.ValidateProposal(blockProposal)
	if model.IsInvalidBlockError(err) {
		f.log.Warn().Err(err).Hex("block_id", logging.ID(blockProposal.Block.BlockID)).
			Msg("invalid proposal")
		return nil
	}
	if errors.Is(err, model.ErrViewForUnknownEpoch) {
		f.log.Warn().
			Hex("block_id", logging.ID(blockProposal.Block.BlockID)).
			Hex("qc_block_id", logging.ID(blockProposal.Block.QC.BlockID)).
			Uint64("block_view", blockProposal.Block.View).
			Msg("proposal for unknown epoch")
	}
	if errors.Is(err, model.ErrUnverifiableBlock) {
		f.log.Warn().
			Hex("block_id", logging.ID(blockProposal.Block.BlockID)).
			Hex("qc_block_id", logging.ID(blockProposal.Block.QC.BlockID)).
			Uint64("block_view", blockProposal.Block.View).
			Msg("unverifiable proposal")

		// even if the block is unverifiable because the QC has been
		// pruned, it still needs to be added to the forks, otherwise,
		// a new block with a QC to this block will fail to be added
		// to forks and crash the event loop.
	} else if err != nil {
		return fmt.Errorf("cannot validate block proposal %x: %w", blockProposal.Block.BlockID, err)
	}

	err = f.finalizationLogic.AddProposal(blockProposal)
	if err != nil {
		return fmt.Errorf("finalization logic cannot process block proposal %x: %w", blockProposal.Block.BlockID, err)
	}

	return nil
}
