package recovery

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Follower recovers the HotStuff state for a follower instance.
// It reads the pending blocks from storage and pass them to the input Finalizer
// instance to recover its state from before the restart.
func Follower(
	log zerolog.Logger,
	finalizer forks.Finalizer,
	validator hotstuff.Validator,
	finalized *flow.Header,
	pending []*flow.Header,
) error {
	return Recover(log, finalized, pending, validator, func(proposal *model.Proposal) error {
		// add it to finalizer
		err := finalizer.AddBlock(proposal.Block)
		if err != nil {
			return fmt.Errorf("could not add block to finalizer: %w", err)
		}
		log.Debug().
			Uint64("block_view", proposal.Block.View).
			Hex("block_id", proposal.Block.BlockID[:]).
			Msg("block recovered")
		return nil
	})
}
