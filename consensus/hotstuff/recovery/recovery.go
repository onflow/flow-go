package recovery

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type ForksRecovery struct {
	forks          hotstuff.Forks
	voteAggregator hotstuff.VoteAggregator
	validator      hotstuff.Validator
	log            zerolog.Logger
}

func NewForksRecovery(log zerolog.Logger, forks hotstuff.Forks, voteAggregator hotstuff.VoteAggregator, validator hotstuff.Validator) (*ForksRecovery, error) {
	return &ForksRecovery{
		log:            log,
		forks:          forks,
		voteAggregator: voteAggregator,
		validator:      validator,
	}, nil
}

// SubmitProposal converts the header into a proposal, verifies it and add it to Forks
func (fr *ForksRecovery) SubmitProposal(proposalHeader *flow.Header, parentView uint64) error {
	// convert the header into a proposal
	proposal := model.ProposalFromFlow(proposalHeader, parentView)

	// verify the proposal
	err := fr.validator.ValidateProposal(proposal)
	if errors.Is(err, model.ErrorInvalidBlock{}) {
		fr.log.Warn().
			Hex("block_id", logging.ID(proposal.Block.BlockID)).
			Err(err).
			Msg("invalid proposal")
		return nil
	}
	if errors.Is(err, model.ErrUnverifiableBlock) {
		fr.log.Warn().
			Hex("block_id", logging.ID(proposal.Block.BlockID)).
			Hex("qc_block_id", logging.ID(proposal.Block.QC.BlockID)).
			Msg("unverifiable proposal")

		// even if the block is unverifiable because the QC has been
		// pruned, it still needs to be added to the forks, otherwise,
		// a new block with a QC to this block will fail to be added
		// to forks and crash the event loop.
	} else if err != nil {
		return fmt.Errorf("cannot validate proposal (%x): %w", proposal.Block.BlockID, err)
	}

	// add it to forks
	err = fr.forks.AddBlock(proposal.Block)
	if err != nil {
		return fmt.Errorf("could not add block to forks: %w", err)
	}

	// recovery the proposer's vote
	_ = fr.voteAggregator.StoreProposerVote(proposal.ProposerVote())

	return nil
}
