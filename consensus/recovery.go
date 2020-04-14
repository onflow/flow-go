package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/recovery"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Recover reads the unfinalized blocks from storage and pass them to the input Forks instance
// to recover its state for the restart.
func Recover(
	log zerolog.Logger,
	forks hotstuff.Forks,
	validator hotstuff.Validator,
	headers storage.Headers,
	state protocol.State,
) error {

	// create the recovery component
	recovery, err := recovery.NewForksRecovery(log, forks, validator)
	if err != nil {
		return fmt.Errorf("can't create ForksRecovery instance: %w", err)
	}

	unfinalized, err := state.Final().Unfinalized()
	if err != nil {
		return fmt.Errorf("can't find unfinalized block ids: %w", err)
	}

	blocks := make(map[flow.Identifier]*flow.Header, len(unfinalized)+1)

	// add all unfinalized blocks to Forks
	for _, unfinalizedBlockID := range unfinalized {
		// find the header from storage
		header, err := headers.ByBlockID(unfinalizedBlockID)
		if err != nil {
			return fmt.Errorf("could not find the header of the unfinalized block by block ID %x: %w", unfinalizedBlockID, err)
		}
		blocks[header.ID()] = header

		// parent must exist in storage, because the index has the parent ID.
		parent, ok := blocks[header.ParentID]
		if !ok {
			return fmt.Errorf("could not find the parent block %x for header %x", header.ParentID, header.ID())
		}

		// add this proposal to Forks to recover
		err = recovery.SubmitProposal(header, parent.View)
		if err != nil {
			return fmt.Errorf("can't recover this proposal: %w", err)
		}
	}

	return nil
}
