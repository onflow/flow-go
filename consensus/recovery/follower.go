package recovery

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Follower recovers the hotstuff state for consensus follower nodes.
// It reads the pending blocks from storage and pass them to the input Finalizer instance
// to recover its state form the restart.
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
