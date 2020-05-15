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

func Recover(log zerolog.Logger, finalized *flow.Header, pending []*flow.Header, validator hotstuff.Validator, onProposal func(*model.Proposal) error) error {
	blocks := make(map[flow.Identifier]*flow.Header, len(pending)+1)

	// finalized is the root
	blocks[finalized.ID()] = finalized

	log.Info().Int("total", len(pending)).Msgf("recovery started")

	// add all pending blocks to forks
	for _, header := range pending {
		blocks[header.ID()] = header

		// parent must exist in storage, because the index has the parent ID
		parent, ok := blocks[header.ParentID]
		if !ok {
			return fmt.Errorf("could not find the parent block %x for header %x", header.ParentID, header.ID())
		}

		// convert the header into a proposal
		proposal := model.ProposalFromFlow(header, parent.View)

		// verify the proposal
		err := validator.ValidateProposal(proposal)
		if errors.Is(err, model.ErrorInvalidBlock{}) {
			log.Warn().
				Hex("block_id", logging.ID(proposal.Block.BlockID)).
				Err(err).
				Msg("invalid proposal")
			return nil
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
