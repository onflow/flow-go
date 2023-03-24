package recovery

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/logging"
)

// Recover implements the core logic for recovering HotStuff state after a restart.
// It accepts the finalized block and a list of pending blocks that have been
// received but not finalized, and that share the latest finalized block as a common
// ancestor.
func Recover(log zerolog.Logger, pending []*flow.Header, validator hotstuff.Validator, onProposal func(*model.Proposal) error) error {
	log.Info().Int("total", len(pending)).Msgf("recovery started")

	// add all pending blocks to forks
	for _, header := range pending {

		// convert the header into a proposal
		proposal := model.ProposalFromFlow(header)

		// verify the proposal
		err := validator.ValidateProposal(proposal)
		if model.IsInvalidBlockError(err) {
			log.Warn().
				Hex("block_id", logging.ID(proposal.Block.BlockID)).
				Err(err).
				Msg("invalid proposal")
			continue
		}
		if errors.Is(err, model.ErrUnverifiableBlock) {
			log.Warn().
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

		err = onProposal(proposal)
		if err != nil {
			return fmt.Errorf("cannot recover proposal: %w", err)
		}
	}

	log.Info().Msgf("recovery completed")

	return nil
}
