package recovery

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Follower recovers the HotStuff state for a follower instance.
// It reads the pending blocks from storage and pass them to the input Forks
// instance to recover its state from before the restart.
func Follower(
	log zerolog.Logger,
	forks hotstuff.Forks,
	validator hotstuff.Validator,
	finalized *flow.Header,
	pending []*flow.Header,
) error {
	return Recover(log, finalized, pending, validator, func(proposal *model.Proposal) error {
		// add it to forks
		err := forks.AddProposal(proposal)
		if err != nil {
			return fmt.Errorf("could not add block to forks: %w", err)
		}
		log.Debug().
			Uint64("block_view", proposal.Block.View).
			Hex("block_id", proposal.Block.BlockID[:]).
			Msg("block recovered")
		return nil
	})
}
