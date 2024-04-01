package ingestion

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/ingestion/block_queue"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Core connects the execution components
// when it receives blocks and collections, it forwards them to the block queue.
// when the block queue decides to execute blocks, it forwards to the executor for execution
// when the block queue decides to fetch missing collections, it forwards to the collection fetcher
// when a block is executed, it notifies the block queue and forwards to execution state to save them.
type Core struct {
	unit *engine.Unit // for async block execution

	log zerolog.Logger

	// state machine
	blockQueue  *block_queue.BlockQueue
	throttle    Throttle // for throttling blocks to be added to the block queue
	execState   state.ExecutionState
	stopControl *stop.StopControl // decide whether to execute a block or not

	// data storage
	headers     storage.Headers
	blocks      storage.Blocks
	collections storage.Collections

	// computation, data fetching, events
	executor          BlockExecutor
	collectionFetcher CollectionFetcher
	eventConsumer     EventConsumer
}

type Throttle interface {
	Init(processables chan<- flow.Identifier) error
	OnBlock(blockID flow.Identifier) error
	OnBlockExecuted(blockID flow.Identifier, height uint64) error
}

type BlockExecutor interface {
	ExecuteBlock(ctx context.Context, block *entity.ExecutableBlock) (*execution.ComputationResult, error)
}

type EventConsumer interface {
	BeforeComputationResultSaved(ctx context.Context, result *execution.ComputationResult)
	OnComputationResultSaved(ctx context.Context, result *execution.ComputationResult) string
}

func NewCore(
	logger zerolog.Logger,
	throttle Throttle,
	execState state.ExecutionState,
	stopControl *stop.StopControl,
	headers storage.Headers,
	blocks storage.Blocks,
	collections storage.Collections,
	executor BlockExecutor,
	collectionFetcher CollectionFetcher,
	eventConsumer EventConsumer,
) *Core {
	return &Core{
		log:               logger.With().Str("engine", "ingestion_core").Logger(),
		unit:              engine.NewUnit(),
		throttle:          throttle,
		execState:         execState,
		blockQueue:        block_queue.NewBlockQueue(logger),
		stopControl:       stopControl,
		headers:           headers,
		blocks:            blocks,
		collections:       collections,
		executor:          executor,
		collectionFetcher: collectionFetcher,
		eventConsumer:     eventConsumer,
	}
}

func (e *Core) Ready() <-chan struct{} {
	if e.stopControl.IsExecutionStopped() {
		return e.unit.Ready()
	}

	e.launchWorkerToConsumeThrottledBlocks()

	return e.unit.Ready()
}

func (e *Core) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Core) OnBlock(header *flow.Header, qc *flow.QuorumCertificate) {
	// qc.Block is equivalent to header.ID()
	err := e.throttle.OnBlock(qc.BlockID)
	if err != nil {
		e.log.Fatal().Err(err).Msgf("error processing block %v (qc.BlockID: %v, blockID: %v)",
			header.Height, qc.BlockID, header.ID())
	}
}

func (e *Core) OnCollection(col *flow.Collection) {
	err := e.onCollection(col)
	if err != nil {
		e.log.Fatal().Err(err).Msgf("error processing collection: %v", col.ID())
	}
}

func (e *Core) launchWorkerToConsumeThrottledBlocks() {
	// processables are throttled blocks
	processables := make(chan flow.Identifier, 10000)

	// running worker in the background to consume
	// processables blocks which are throttled,
	// and forward them to the block queue for processing
	e.unit.Launch(func() {
		e.log.Info().Msgf("starting worker to consume throttled blocks")
		err := e.forwardProcessableToHandler(processables)
		if err != nil {
			e.log.Fatal().Err(err).Msg("fail to process block")
		}
	})

	e.log.Info().Msg("initializing throttle engine")

	err := e.throttle.Init(processables)
	if err != nil {
		e.log.Fatal().Err(err).Msg("fail to initialize throttle engine")
	}

	e.log.Info().Msgf("throttle engine initialized")
}

func (e *Core) forwardProcessableToHandler(
	processables <-chan flow.Identifier,
) error {
	for blockID := range processables {
		err := e.onProcessableBlock(blockID)
		if err != nil {
			return fmt.Errorf("could not process block: %w", err)
		}
	}

	return nil
}

func (e *Core) onProcessableBlock(blockID flow.Identifier) error {
	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get block: %w", err)
	}

	// skip if stopControl tells to skip
	if !e.stopControl.ShouldExecuteBlock(header) {
		return nil
	}

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

	missingColls, executables, err := e.enqueuBlock(block, blockID)
	if err != nil {
		return fmt.Errorf("failed to enqueue block %v: %w", blockID, err)
	}

	e.executeConcurrently(executables)

	err = e.fetch(missingColls)
	if err != nil {
		return fmt.Errorf("failed to fetch missing collections: %w", err)
	}

	return nil
}

func (e *Core) enqueuBlock(block *flow.Block, blockID flow.Identifier) (
	[]*block_queue.MissingCollection,
	[]*entity.ExecutableBlock,
	error,
) {
	lg := e.log.With().
		Hex("block_id", blockID[:]).
		Uint64("height", block.Header.Height).
		Logger()

	lg.Info().Msg("handling new block")

	parentCommitment, err := e.execState.StateCommitmentByBlockID(block.Header.ParentID)

	if err == nil {
		// the parent block is an executed block.
		missingColls, executables, err := e.blockQueue.HandleBlock(block, &parentCommitment)
		if err != nil {
			return nil, nil, fmt.Errorf("unexpected error while adding block to block queue: %w", err)
		}

		lg.Info().Bool("parent_is_executed", true).
			Int("missing_col", len(missingColls)).
			Int("executables", len(executables)).
			Msgf("block is enqueued")

		return missingColls, executables, nil
	}

	// handle exception
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, nil, fmt.Errorf("failed to get state commitment for parent block %v of block %v (height: %v): %w",
			block.Header.ParentID, blockID, block.Header.Height, err)
	}

	// the parent block is an unexecuted block.
	// we can enqueue the block without providing the state commitment
	missingColls, executables, err := e.blockQueue.HandleBlock(block, nil)
	if err != nil {
		if !errors.Is(err, block_queue.ErrMissingParent) {
			return nil, nil, fmt.Errorf("unexpected error while adding block to block queue: %w", err)
		}

		// if parent is missing, there are two possibilities:
		// 1) parent was never enqueued to block queue
		// 2) parent was enqueued, but it has been executed and removed from the block queue
		// however, actually 1) is not possible 2) is the only possible case here, why?
		// because forwardProcessableToHandler guarantees we always enqueue a block before its child,
		// which means when HandleBlock is called with a block, then its parent block must have been
		// called with HandleBlock already. Therefore, 1) is not possible.
		// And the reason 2) is possible is because the fact that its parent block is missing
		// might be outdated since OnBlockExecuted might be called concurrently in a different thread.
		// it means OnBlockExecuted is called in a different thread after us getting the parent commit
		// and before HandleBlock was called, therefore, we should re-enqueue the block with the
		// parent commit. It's necessary to check again whether the parent block is executed after the call.
		lg.Warn().Msgf(
			"block is missing parent block, re-enqueueing %v (parent: %v)",
			blockID, block.Header.ParentID,
		)

		parentCommitment, err := e.execState.StateCommitmentByBlockID(block.Header.ParentID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get parent state commitment when re-enqueue block %v (parent: %v): %w",
				blockID, block.Header.ParentID, err)
		}

		// now re-enqueue the block with parent commit
		missing, execs, err := e.blockQueue.HandleBlock(block, &parentCommitment)
		if err != nil {
			return nil, nil, fmt.Errorf("unexpected error while reenqueue block to block queue: %w", err)
		}

		missingColls = flow.Deduplicate(append(missingColls, missing...))
		executables = flow.Deduplicate(append(executables, execs...))
	}

	lg.Info().Bool("parent_is_executed", false).
		Int("missing_col", len(missingColls)).
		Int("executables", len(executables)).
		Msgf("block is enqueued")

	return missingColls, executables, nil
}

func (e *Core) onBlockExecuted(
	block *entity.ExecutableBlock,
	computationResult *execution.ComputationResult,
	startedAt time.Time,
) error {
	commit := computationResult.CurrentEndState()

	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()

	go func() {
		defer wg.Done()
		e.eventConsumer.BeforeComputationResultSaved(e.unit.Ctx(), computationResult)
	}()

	err := e.execState.SaveExecutionResults(e.unit.Ctx(), computationResult)
	if err != nil {
		return fmt.Errorf("cannot persist execution state: %w", err)
	}

	// must call OnBlockExecuted AFTER saving the execution result to storage
	// because when enqueuing a block, we rely on execState.StateCommitmentByBlockID
	// to determine whether a block has been executed or not.
	executables, err := e.blockQueue.OnBlockExecuted(block.ID(), commit)
	if err != nil {
		return fmt.Errorf("unexpected error while marking block as executed: %w", err)
	}

	e.stopControl.OnBlockExecuted(block.Block.Header)

	// notify event consumer so that the event consumer can do tasks
	// such as broadcasting or uploading the result
	logs := e.eventConsumer.OnComputationResultSaved(e.unit.Ctx(), computationResult)

	receipt := computationResult.ExecutionReceipt
	e.log.Info().
		Hex("block_id", logging.Entity(block)).
		Uint64("height", block.Block.Header.Height).
		Int("collections", len(block.CompleteCollections)).
		Hex("parent_block", block.Block.Header.ParentID[:]).
		Int("collections", len(block.Block.Payload.Guarantees)).
		Hex("start_state", block.StartState[:]).
		Hex("final_state", commit[:]).
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("result_id", logging.Entity(receipt.ExecutionResult)).
		Hex("execution_data_id", receipt.ExecutionResult.ExecutionDataID[:]).
		Bool("state_changed", commit != *block.StartState).
		Uint64("num_txs", nonSystemTransactionCount(receipt.ExecutionResult)).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Str("logs", logs). // broadcasted
		Msgf("block executed")

	// we ensures that the child blocks are only executed after the execution result of
	// its parent block has been successfully saved to storage.
	// this ensures OnBlockExecuted would not be called with blocks in a wrong order, such as
	// OnBlockExecuted(childBlock) being called before OnBlockExecuted(parentBlock).
	e.executeConcurrently(executables)

	return nil
}

func (e *Core) onCollection(col *flow.Collection) error {
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
	executables, err := e.blockQueue.HandleCollection(col)
	if err != nil {
		return fmt.Errorf("unexpected error while adding collection to block queue")
	}

	e.executeConcurrently(executables)

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

// execute block concurrently
func (e *Core) executeConcurrently(executables []*entity.ExecutableBlock) {
	for _, executable := range executables {
		func(executable *entity.ExecutableBlock) {
			e.unit.Launch(func() {
				e.log.Info().Msgf("starting worker to consume throttled blocks")
				err := e.execute(executable)
				if err != nil {
					e.log.Error().Err(err).Msgf("failed to execute block %v", executable.Block.ID())
				}
			})
		}(executable)
	}
}

func (e *Core) execute(executable *entity.ExecutableBlock) error {
	if !e.stopControl.ShouldExecuteBlock(executable.Block.Header) {
		return nil
	}

	e.log.Info().
		Hex("block_id", logging.Entity(executable)).
		Uint64("height", executable.Block.Header.Height).
		Int("collections", len(executable.CompleteCollections)).
		Msgf("executing block")

	startedAt := time.Now()

	result, err := e.executor.ExecuteBlock(e.unit.Ctx(), executable)
	if err != nil {
		return fmt.Errorf("failed to execute block %v: %w", executable.Block.ID(), err)
	}

	err = e.onBlockExecuted(executable, result, startedAt)
	if err != nil {
		return fmt.Errorf("failed to handle execution result of block %v: %w", executable.Block.ID(), err)
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
