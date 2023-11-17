package loader

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// deprecated. Storehouse is going to use unfinalized loader instead
type UnexecutedLoader struct {
	log       zerolog.Logger
	state     protocol.State
	headers   storage.Headers // see comments on getHeaderByHeight for why we need it
	execState state.ExecutionState
}

func NewUnexecutedLoader(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	execState state.ExecutionState,
) *UnexecutedLoader {
	return &UnexecutedLoader{
		log:       log.With().Str("component", "ingestion_engine_unexecuted_loader").Logger(),
		state:     state,
		headers:   headers,
		execState: execState,
	}
}

func (e *UnexecutedLoader) LoadUnexecuted(ctx context.Context) ([]flow.Identifier, error) {
	// saving an executed block is currently not transactional, so it's possible
	// the block is marked as executed but the receipt might not be saved during a crash.
	// in order to mitigate this problem, we always re-execute the last executed and finalized
	// block.
	// there is an exception, if the last executed block is a root block, then don't execute it,
	// because the root has already been executed during bootstrapping phase. And re-executing
	// a root block will fail, because the root block doesn't have a parent block, and could not
	// get the result of it.
	// TODO: remove this, when saving a executed block is transactional
	lastExecutedHeight, lastExecutedID, err := e.execState.GetHighestExecutedBlockID(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get last executed: %w", err)
	}

	last, err := e.headers.ByBlockID(lastExecutedID)
	if err != nil {
		return nil, fmt.Errorf("could not get last executed final by ID: %w", err)
	}

	// don't reload root block
	rootBlock, err := e.state.Params().SealedRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve root block: %w", err)
	}

	blockIDs := make([]flow.Identifier, 0)
	isRoot := rootBlock.ID() == last.ID()
	if !isRoot {
		executed, err := state.IsBlockExecuted(ctx, e.execState, lastExecutedID)
		if err != nil {
			return nil, fmt.Errorf("cannot check is last exeucted final block has been executed %v: %w", lastExecutedID, err)
		}
		if !executed {
			// this should not happen, but if it does, execution should still work
			e.log.Warn().
				Hex("block_id", lastExecutedID[:]).
				Msg("block marked as highest executed one, but not executable - internal inconsistency")

			blockIDs = append(blockIDs, lastExecutedID)
		}
	}

	finalized, pending, err := e.unexecutedBlocks(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not reload unexecuted blocks: %w", err)
	}

	unexecuted := append(finalized, pending...)

	log := e.log.With().
		Int("total", len(unexecuted)).
		Int("finalized", len(finalized)).
		Int("pending", len(pending)).
		Uint64("last_executed", lastExecutedHeight).
		Hex("last_executed_id", lastExecutedID[:]).
		Logger()

	log.Info().Msg("reloading unexecuted blocks")

	for _, blockID := range unexecuted {
		blockIDs = append(blockIDs, blockID)
		e.log.Debug().Hex("block_id", blockID[:]).Msg("reloaded block")
	}

	log.Info().Msg("all unexecuted have been successfully reloaded")

	return blockIDs, nil
}

func (e *UnexecutedLoader) unexecutedBlocks(ctx context.Context) (
	finalized []flow.Identifier,
	pending []flow.Identifier,
	err error,
) {
	// pin the snapshot so that finalizedUnexecutedBlocks and pendingUnexecutedBlocks are based
	// on the same snapshot.
	snapshot := e.state.Final()

	finalized, err = e.finalizedUnexecutedBlocks(ctx, snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read finalized unexecuted blocks")
	}

	pending, err = e.pendingUnexecutedBlocks(ctx, snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read pending unexecuted blocks")
	}

	return finalized, pending, nil
}

func (e *UnexecutedLoader) finalizedUnexecutedBlocks(ctx context.Context, finalized protocol.Snapshot) (
	[]flow.Identifier,
	error,
) {
	// get finalized height
	final, err := finalized.Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block: %w", err)
	}

	// find the first unexecuted and finalized block
	// We iterate from the last finalized, check if it has been executed,
	// if not, keep going to the lower height, until we find an executed
	// block, and then the next height is the first unexecuted.
	// If there is only one finalized, and it's executed (i.e. root block),
	// then the firstUnexecuted is a unfinalized block, which is ok,
	// because the next loop will ensure it only iterates through finalized
	// blocks.
	lastExecuted := final.Height

	// dynamically bootstrapped execution node will reload blocks from
	// [sealedRoot.Height + 1, finalizedRoot.Height] and execute them on startup.
	rootBlock, err := e.state.Params().SealedRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve root block: %w", err)
	}

	for ; lastExecuted > rootBlock.Height; lastExecuted-- {
		header, err := e.getHeaderByHeight(lastExecuted)
		if err != nil {
			return nil, fmt.Errorf("could not get header at height: %v, %w", lastExecuted, err)
		}

		executed, err := state.IsBlockExecuted(ctx, e.execState, header.ID())
		if err != nil {
			return nil, fmt.Errorf("could not check whether block is executed: %w", err)
		}

		if executed {
			break
		}
	}

	firstUnexecuted := lastExecuted + 1

	unexecuted := make([]flow.Identifier, 0)

	// starting from the first unexecuted block, go through each unexecuted and finalized block
	// reload its block to execution queues
	for height := firstUnexecuted; height <= final.Height; height++ {
		header, err := e.getHeaderByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("could not get header at height: %v, %w", height, err)
		}

		unexecuted = append(unexecuted, header.ID())
	}

	e.log.Info().
		Uint64("last_finalized", final.Height).
		Uint64("last_finalized_executed", lastExecuted).
		Uint64("sealed_root_height", rootBlock.Height).
		Hex("sealed_root_id", logging.Entity(rootBlock)).
		Uint64("first_unexecuted", firstUnexecuted).
		Int("total_finalized_unexecuted", len(unexecuted)).
		Msgf("finalized unexecuted blocks")

	return unexecuted, nil
}

func (e *UnexecutedLoader) pendingUnexecutedBlocks(ctx context.Context, finalized protocol.Snapshot) (
	[]flow.Identifier,
	error,
) {
	pendings, err := finalized.Descendants()
	if err != nil {
		return nil, fmt.Errorf("could not get pending blocks: %w", err)
	}

	unexecuted := make([]flow.Identifier, 0)

	for _, pending := range pendings {
		executed, err := state.IsBlockExecuted(ctx, e.execState, pending)
		if err != nil {
			return nil, fmt.Errorf("could not check block executed or not: %w", err)
		}

		if !executed {
			unexecuted = append(unexecuted, pending)
		}
	}

	return unexecuted, nil
}

// if the EN is dynamically bootstrapped, the finalized blocks at height range:
// [ sealedRoot.Height, finalizedRoot.Height - 1] can not be retrieved from
// protocol state, but only from headers
func (e *UnexecutedLoader) getHeaderByHeight(height uint64) (*flow.Header, error) {
	// we don't use protocol state because for dynamic boostrapped execution node
	// the last executed and sealed block is below the finalized root block
	return e.headers.ByHeight(height)
}
