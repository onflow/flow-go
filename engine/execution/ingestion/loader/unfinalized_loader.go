package loader

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type UnfinalizedLoader struct {
	log       zerolog.Logger
	state     protocol.State
	headers   storage.Headers
	execState state.FinalizedExecutionState
}

// NewUnfinalizedLoader creates a new loader that loads all unfinalized and validated blocks
func NewUnfinalizedLoader(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	execState state.FinalizedExecutionState,
) *UnfinalizedLoader {
	return &UnfinalizedLoader{
		log:       log.With().Str("component", "ingestion_engine_unfinalized_loader").Logger(),
		state:     state,
		headers:   headers,
		execState: execState,
	}
}

// LoadUnexecuted loads all unfinalized and validated blocks
// any error returned are exceptions
func (e *UnfinalizedLoader) LoadUnexecuted(ctx context.Context) ([]flow.Identifier, error) {
	lastExecuted := e.execState.GetHighestFinalizedExecuted()

	// get finalized height
	finalized := e.state.Final()
	final, err := finalized.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block: %w", err)
	}

	lg := e.log.With().
		Uint64("last_finalized", final.Height).
		Uint64("last_finalized_executed", lastExecuted).
		Logger()

	lg.Info().Msgf("start loading unfinalized blocks")

	// dynamically bootstrapped execution node will have highest finalized executed as sealed root,
	// which is lower than finalized root. so we will reload blocks from
	// [sealedRoot.Height + 1, finalizedRoot.Height] and execute them on startup.
	unexecutedFinalized := make([]flow.Identifier, 0)

	// starting from the first unexecuted block, go through each unexecuted and finalized block
	// reload its block to execution queues
	// loading finalized blocks
	for height := lastExecuted + 1; height <= final.Height; height++ {
		finalizedID, err := e.headers.BlockIDByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header at height: %v, %w", height, err)
		}

		unexecutedFinalized = append(unexecutedFinalized, finalizedID)
	}

	// loaded all pending blocks
	pendings, err := finalized.Descendants()
	if err != nil {
		return nil, fmt.Errorf("could not get descendants of finalized block: %w", err)
	}

	unexecuted := append(unexecutedFinalized, pendings...)

	lg.Info().
		// Uint64("sealed_root_height", rootBlock.Height).
		// Hex("sealed_root_id", logging.Entity(rootBlock)).
		Int("total_finalized_unexecuted", len(unexecutedFinalized)).
		Int("total_unexecuted", len(unexecuted)).
		Msgf("finalized unexecuted blocks")

	return unexecuted, nil
}
