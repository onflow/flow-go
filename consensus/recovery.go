package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/recovery"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Recover reads the pending blocks from storage and pass them to the input Forks instance
// to recover its state for the restart.
func Recover(
	log zerolog.Logger,
	forks hotstuff.Forks,
	validator hotstuff.Validator,
	finalized *flow.Header,
	pending []*flow.Header,
) error {

	// create the recovery component
	recovery, err := recovery.NewForksRecovery(log, forks, validator)
	if err != nil {
		return fmt.Errorf("could not initialize recovery: %w", err)
	}

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

		// add this proposal to forks to recover
		err = recovery.SubmitProposal(header, parent.View)
		if err != nil {
			return fmt.Errorf("can't recover this proposal: %w", err)
		}
	}

	log.Info().Msgf("recovery completed")

	return nil
}
