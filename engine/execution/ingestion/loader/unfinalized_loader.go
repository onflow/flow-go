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
	headers   storage.Headers // see comments on getHeaderByHeight for why we need it
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

	// TODO: dynamically bootstrapped execution node will reload blocks from
	unexecutedFinalized := make([]flow.Identifier, 0)

	// starting from the first unexecuted block, go through each unexecuted and finalized block
	// reload its block to execution queues
	// loading finalized blocks
	for height := lastExecuted + 1; height <= final.Height; height++ {
		header, err := e.getHeaderByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header at height: %v, %w", height, err)
		}

		unexecutedFinalized = append(unexecutedFinalized, header.ID())

		if len(unexecutedFinalized) > 10_000 {
			e.log.Warn().
				Uint64("total_unexecuted", final.Height-lastExecuted).
				Msg("too many unexecuted finalized blocks, loading the first 10_000")
			break
		}

	}

	// loaded all pending blocks
	pendings, err := finalized.Descendants()
	if err != nil {
		return nil, fmt.Errorf("could not get descendants of finalized block: %w", err)
	}

	unexecuted := append(unexecutedFinalized, pendings...)

	e.log.Info().
		Uint64("last_finalized", final.Height).
		Uint64("last_finalized_executed", lastExecuted).
		// Uint64("sealed_root_height", rootBlock.Height).
		// Hex("sealed_root_id", logging.Entity(rootBlock)).
		Int("total_finalized_unexecuted", len(unexecutedFinalized)).
		Int("total_unexecuted", len(unexecuted)).
		Msgf("finalized unexecuted blocks")

	return unexecuted, nil
}

// if the EN is dynamically bootstrapped, the finalized blocks at height range:
// [ sealedRoot.Height, finalizedRoot.Height - 1] can not be retrieved from
// protocol state, but only from headers
func (e *UnfinalizedLoader) getHeaderByHeight(height uint64) (*flow.Header, error) {
	// we don't use protocol state because for dynamic boostrapped execution node
	// the last executed and sealed block is below the finalized root block
	return e.headers.ByHeight(height)
}
