package ingestion

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/ingestion/block_queue"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/storage"
)

type Core struct {
	log zerolog.Logger

	// state machine
	blockQueue  *block_queue.BlockQueue
	execState   state.ExecutionState
	stopControl *stop.StopControl // decide whether to execute a block or not

	// data storage
	headers     storage.Headers
	blocks      storage.Blocks
	collections storage.Collections

	// computation and data fetching
	executor          BlockExecutor
	collectionFetcher CollectionFetcher
}

type BlockExecutor interface {
	ExecuteBlock(handler ComputationResultHandler, block *entity.ExecutableBlock) error
}

type ComputationResultHandler interface {
	OnBlockExecuted(block *entity.ExecutableBlock, computationResult *execution.ComputationResult)
}

func NewCore(
	logger zerolog.Logger,
	execState state.ExecutionState,
	stopControl *stop.StopControl,
	headers storage.Headers,
	blocks storage.Blocks,
	collections storage.Collections,
	executor BlockExecutor,
	collectionFetcher CollectionFetcher,
) *Core {
	return &Core{
		log:               logger,
		execState:         execState,
		blockQueue:        block_queue.NewBlockQueue(logger),
		stopControl:       stopControl,
		headers:           headers,
		blocks:            blocks,
		collections:       collections,
		executor:          executor,
		collectionFetcher: collectionFetcher,
	}
}

func (e *Core) OnBlock(header *flow.Header) error {
	// skip if stopControl tells to skip
	if !e.stopControl.ShouldExecuteBlock(header) {
		return nil
	}

	blockID := header.ID()

	executed, err := e.execState.IsBlockExecuted(header.Height, blockID)
	if err != nil {
		return fmt.Errorf("could not check whether block %v is executed: %w", blockID, err)
	}

	if executed {
		e.log.Debug().Msg("block has been executed already")
		return nil
	}

	block, err := e.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("failed to get block %s: %w", blockID, err)
	}

	e.log.Info().Hex("block_id", blockID[:]).
		Uint64("height", header.Height).
		Msg("handling new block")

	missingColls, executables, err := e.enqueuBlock(block, blockID)
	if err != nil {
		return fmt.Errorf("failed to enqueue block %v: %w", blockID, err)
	}

	err = e.execute(executables)
	if err != nil {
		return fmt.Errorf("failed to execute block: %w", err)
	}

	err = e.fetch(missingColls)
	if err != nil {
		return fmt.Errorf("failed to fetch missing collections: %w", err)
	}

	return nil
}

func (e *Core) enqueuBlock(block *flow.Block, blockID flow.Identifier) (
	[]*block_queue.MissingCollection, []*entity.ExecutableBlock, error) {
	parentCommitment, err := e.execState.StateCommitmentByBlockID(block.Header.ParentID)

	if err == nil {
		// the parent block is an executed block.
		missingColls, executables, err := e.blockQueue.OnBlock(block, &parentCommitment)
		if err != nil {
			return nil, nil, fmt.Errorf("unexpected error while adding block to block queue: %w", err)
		}

		e.log.Info().Bool("parent_is_executed", true).
			Int("missing_col", len(missingColls)).
			Int("executables", len(executables)).
			Msgf("block %v is enqueued", blockID)

		return missingColls, executables, nil
	}

	// handle exception
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, nil, fmt.Errorf("failed to get parent state commitment for block %v: %w", block.Header.ParentID, err)
	}

	// the parent block is an unexecuted block.
	// Note: the fact that its parent block is unexecuted might be outdated since OnBlockExecuted
	// might be called concurrently in a different thread, it's necessary to check again
	// whether the parent block is executed after the call.
	missingColls, executables, err := e.blockQueue.OnBlock(block, nil)
	if err != nil {
		if !errors.Is(err, block_queue.ErrMissingParent) {
			return nil, nil, fmt.Errorf("unexpected error while adding block to block queue: %w", err)
		}

		// if the error is ErrMissingParent, it means OnBlockExecuted is called after us getting
		// the parent commit and before OnBlock was called, therefore, we should re-enqueue the block
		// with the parent commit.
		e.log.Warn().Msgf(
			"block %v is missing parent block %v, re-enqueueing",
			blockID,
			block.Header.ParentID,
		)

		parentCommitment, err := e.execState.StateCommitmentByBlockID(block.Header.ParentID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get parent state commitment for block %v when re-enqueue block %v: %w",
				block.Header.ParentID, blockID, err)
		}

		// now re-enqueue the block with parent commit
		missing, execs, err := e.blockQueue.OnBlock(block, &parentCommitment)
		if err != nil {
			return nil, nil, fmt.Errorf("unexpected error while reenqueue block to block queue: %w", err)
		}

		missingColls = deduplicate(append(missingColls, missing...))
		executables = deduplicate(append(executables, execs...))

		e.log.Info().Msgf("successfully reenqueued block %v", blockID)
	}

	e.log.Info().Bool("parent_is_executed", false).
		Int("missing_col", len(missingColls)).
		Int("executables", len(executables)).
		Msgf("block %v is enqueued", blockID)

	return missingColls, executables, nil
}

func (e *Core) OnBlockExecuted(block *entity.ExecutableBlock, computationResult *execution.ComputationResult) {
	err := e.onBlockExecuted(block, computationResult)
	if err != nil {
		// TODO: use irrecoverable error
		e.log.Fatal().Err(err).Msg("unexpected error while handling block execution")
	}
}

func (e *Core) onBlockExecuted(block *entity.ExecutableBlock, computationResult *execution.ComputationResult) error {
	commit := computationResult.CurrentEndState()

	// we notify the block queue before saving the results to database,
	// so that it reduce the chance of race condition when OnBlock is called with
	// parent commit which might get outdated.
	executables, err := e.blockQueue.OnBlockExecuted(block.ID(), commit)
	if err != nil {
		return fmt.Errorf("unexpected error while marking block as executed: %w", err)
	}

	// TODO: use app level context
	err = e.execState.SaveExecutionResults(context.Background(), computationResult)
	if err != nil {
		return fmt.Errorf("cannot persist execution state: %w", err)
	}

	e.stopControl.OnBlockExecuted(block.Block.Header)

	// we ensures that the child blocks are only executed after the execution result of
	// its parent block has been successfully saved to storage.
	// this ensures OnBlockExecuted would not be called with blocks in a wrong order, such as
	// OnBlockExecuted(childBlock) being called before OnBlockExecuted(parentBlock).
	err = e.execute(executables)
	if err != nil {
		return fmt.Errorf("failed to execute block when receiving %v: %w", block.ID(), err)
	}

	return nil
}

func (e *Core) OnCollection(col *flow.Collection) error {
	// EN might request a collection from multiple collection nodes,
	// therefore might receive multiple copies of the same collection.
	// we only need to store it once.
	err := storeCollectionIfMissing(e.collections, col)
	if err != nil {
		return fmt.Errorf("failed to store collection %v: %w", col.ID(), err)
	}

	// if the collection is a duplication, it's still good to add it to the block queue,
	// because chances are the collection was stored before a restart, and
	// is not in the queue after the restart.
	// adding it to the queue ensures we don't miss any collection.
	// since the queue's state is in memory, processing a duplicated collection should be
	// a fast no-op, and won't return any executable blocks.
	executables, err := e.blockQueue.OnCollection(col)
	if err != nil {
		return fmt.Errorf("unexpected error while adding collection to block queue")
	}

	err = e.execute(executables)
	if err != nil {
		return fmt.Errorf("failed to execute block when receiving collection %v: %w", col.ID(), err)
	}

	return nil
}

func storeCollectionIfMissing(collections storage.Collections, col *flow.Collection) error {
	_, err := collections.ByID(col.ID())
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to get collection %v: %w", col.ID(), err)
		}

		err := collections.Store(col)
		if err != nil {
			return fmt.Errorf("failed to store collection %v: %w", col.ID(), err)
		}
	}

	return nil
}

func (e *Core) execute(executables []*entity.ExecutableBlock) error {
	for _, executable := range executables {
		if !e.stopControl.ShouldExecuteBlock(executable.Block.Header) {
			continue
		}

		// passing the block and itself to the executor so that when the executor
		// finish executing the block, it can call OnBlockExecuted to notify the ingester
		err := e.executor.ExecuteBlock(e, executable)
		if err != nil {
			return fmt.Errorf("failed to execute block %v: %w", executable.Block.ID(), err)
		}
	}

	return nil
}

func (e *Core) fetch(missingColls []*block_queue.MissingCollection) error {
	for _, col := range missingColls {
		err := e.collectionFetcher.FetchCollection(col.BlockID, col.Height, col.Guarantee)
		if err != nil {
			return fmt.Errorf("failed to fetch collection %v for block %v (height: %v): %w",
				col.Guarantee.ID(), col.BlockID, col.Height, err)
		}
	}

	if len(missingColls) > 0 {
		e.collectionFetcher.Force()
	}

	return nil
}

type IDEntity interface {
	ID() flow.Identifier
}

// deduplicate entities in a slice by the ID method
func deduplicate[T IDEntity](entities []T) []T {
	seen := make(map[flow.Identifier]struct{}, len(entities))
	result := make([]T, 0, len(entities))

	for _, entity := range entities {
		id := entity.ID()
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, entity)
	}

	return result
}
