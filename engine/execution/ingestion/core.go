package ingestion

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/ingestion/block_queue"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// MaxProcessableBlocks is the maximum number of blocks that is queued to be processed
const MaxProcessableBlocks = 10000

// MaxConcurrentBlockExecutor is the maximum number of concurrent block executors
const MaxConcurrentBlockExecutor = 5

// Core connects the execution components
// when it receives blocks and collections, it forwards them to the block queue.
// when the block queue decides to execute blocks, it forwards to the executor for execution
// when the block queue decides to fetch missing collections, it forwards to the collection fetcher
// when a block is executed, it notifies the block queue and forwards to execution state to save them.
type Core struct {
	*component.ComponentManager

	log zerolog.Logger

	// when a block is received, it is first pushed to the processables channel, and then the worker will
	// fetch the collections and forward it to the block queue.
	// once the data is fetched, and its parent block is executed, then the block is ready to be executed, it
	// will be pushed to the blockExecutors channel, and the worker will execute the block.
	// during startup, the throttle will limit the number of blocks to be added to the processables channel.
	// once caught up, the throttle will allow all the remaining blocks to be added to the processables channel.
	processables   chan flow.Identifier         // block IDs that are received and waiting to be processed
	throttle       Throttle                     // to throttle the blocks to be added to processables during startup and catchup
	blockQueue     *block_queue.BlockQueue      // blocks are waiting for the data to be fetched
	blockExecutors chan *entity.ExecutableBlock // blocks that are ready to be executed
	stopControl    *stop.StopControl            // decide whether to execute a block or not and when to stop the execution

	// data storage
	execState   state.ExecutionState
	headers     storage.Headers
	blocks      storage.Blocks
	collections storage.Collections

	// computation, data fetching, events
	executor          BlockExecutor
	collectionFetcher CollectionFetcher
	eventConsumer     EventConsumer
}

// Throttle is used to throttle the blocks to be added to the processables channel
type Throttle interface {
	// Init initializes the throttle with the processables channel to forward the blocks
	Init(processables chan<- flow.Identifier) error
	// OnBlock is called when a block is received, the throttle will check if the execution
	// is falling far behind the finalization, and add the block to the processables channel
	// if it's not falling far behind.
	OnBlock(blockID flow.Identifier) error
	// OnBlockExecuted is called when a block is executed, the throttle will check whether
	// the execution is caught up with the finalization, and allow all the remaining blocks
	// to be added to the processables channel.
	OnBlockExecuted(blockID flow.Identifier, height uint64) error
	// Done stops the throttle, and stop sending new blocks to the processables channel
	Done() error
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
) (*Core, error) {
	e := &Core{
		log:               logger.With().Str("engine", "ingestion_core").Logger(),
		processables:      make(chan flow.Identifier, MaxProcessableBlocks),
		blockExecutors:    make(chan *entity.ExecutableBlock),
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

	err := e.throttle.Init(e.processables)
	if err != nil {
		return nil, fmt.Errorf("fail to initialize throttle engine: %w", err)
	}

	e.log.Info().Msgf("throttle engine initialized")

	builder := component.NewComponentManagerBuilder().AddWorker(e.launchWorkerToHandleBlocks)

	for w := 0; w < MaxConcurrentBlockExecutor; w++ {
		builder.AddWorker(e.launchWorkerToExecuteBlocks)
	}

	e.ComponentManager = builder.Build()

	return e, nil
}

func (e *Core) launchWorkerToHandleBlocks(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	executionStopped := e.stopControl.IsExecutionStopped()

	e.log.Info().Bool("execution_stopped", executionStopped).Msgf("launching worker")

	ready()

	if executionStopped {
		return
	}

	e.launchWorkerToConsumeThrottledBlocks(ctx)
}

func (e *Core) launchWorkerToExecuteBlocks(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case executable := <-e.blockExecutors:
			err := e.execute(ctx, executable)
			if err != nil {
				ctx.Throw(fmt.Errorf("execution ingestion engine failed to execute block %v (%v): %w",
					executable.Block.Header.Height,
					executable.Block.ID(), err))
			}
		}
	}
}

func (e *Core) OnBlock(header *flow.Header, qc *flow.QuorumCertificate) {
	e.log.Debug().
		Hex("block_id", qc.BlockID[:]).Uint64("height", header.Height).
		Msgf("received block")

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

func (e *Core) launchWorkerToConsumeThrottledBlocks(ctx irrecoverable.SignalerContext) {
	// running worker in the background to consume
	// processables blocks which are throttled,
	// and forward them to the block queue for processing
	e.log.Info().Msgf("starting worker to consume throttled blocks")
	defer func() {
		e.log.Info().Msgf("worker to consume throttled blocks stopped")
	}()
	for {
		select {
		case <-ctx.Done():
			// if the engine has shut down, then mark throttle as Done, which
			// will stop sending new blocks to e.processables
			err := e.throttle.Done()
			if err != nil {
				ctx.Throw(fmt.Errorf("execution ingestion engine failed to stop throttle: %w", err))
			}

			// drain the processables
			e.log.Info().Msgf("draining processables")
			close(e.processables)
			for range e.processables {
			}
			e.log.Info().Msgf("finish draining processables")
			return

		case blockID := <-e.processables:
			e.log.Debug().Hex("block_id", blockID[:]).Msg("ingestion core processing block")
			err := e.onProcessableBlock(blockID)
			if err != nil {
				ctx.Throw(fmt.Errorf("execution ingestion engine fail to process block %v: %w", blockID, err))
				return
			}
		}
	}

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
		e.log.Debug().Hex("block_id", blockID[:]).Uint64("height", header.Height).Msg("block has been executed already")
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

	e.log.Debug().
		Hex("block_id", blockID[:]).Uint64("height", header.Height).
		Int("executables", len(executables)).Msgf("executeConcurrently block is executable")
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
	ctx context.Context,
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
		e.eventConsumer.BeforeComputationResultSaved(ctx, computationResult)
	}()

	err := e.execState.SaveExecutionResults(ctx, computationResult)
	if err != nil {
		return fmt.Errorf("cannot persist execution state: %w", err)
	}

	blockID := block.ID()
	lg := e.log.With().
		Hex("block_id", blockID[:]).
		Uint64("height", block.Block.Header.Height).
		Logger()

	lg.Debug().Msgf("execution state saved")

	// must call OnBlockExecuted AFTER saving the execution result to storage
	// because when enqueuing a block, we rely on execState.StateCommitmentByBlockID
	// to determine whether a block has been executed or not.
	executables, err := e.blockQueue.OnBlockExecuted(blockID, commit)
	if err != nil {
		return fmt.Errorf("unexpected error while marking block as executed: %w", err)
	}

	e.stopControl.OnBlockExecuted(block.Block.Header)

	// notify event consumer so that the event consumer can do tasks
	// such as broadcasting or uploading the result
	logs := e.eventConsumer.OnComputationResultSaved(ctx, computationResult)

	receipt := computationResult.ExecutionReceipt
	lg.Info().
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
		Int("executables", len(executables)).
		Msgf("block executed")

	// we ensures that the child blocks are only executed after the execution result of
	// its parent block has been successfully saved to storage.
	// this ensures OnBlockExecuted would not be called with blocks in a wrong order, such as
	// OnBlockExecuted(childBlock) being called before OnBlockExecuted(parentBlock).

	e.executeConcurrently(executables)

	return nil
}

func (e *Core) onCollection(col *flow.Collection) error {
	colID := col.ID()
	e.log.Info().
		Hex("collection_id", colID[:]).
		Msgf("handle collection")
	// EN might request a collection from multiple collection nodes,
	// therefore might receive multiple copies of the same collection.
	// we only need to store it once.
	err := storeCollectionIfMissing(e.collections, col)
	if err != nil {
		return fmt.Errorf("failed to store collection %v: %w", col.ID(), err)
	}

	return e.handleCollection(colID, col)
}

func (e *Core) handleCollection(colID flow.Identifier, col *flow.Collection) error {
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

	e.log.Debug().
		Hex("collection_id", colID[:]).
		Int("executables", len(executables)).Msgf("executeConcurrently: collection is handled, ready to execute block")

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
		select {
		case <-e.ShutdownSignal():
			// if the engine has shut down, then stop executing the block
			return
		case e.blockExecutors <- executable:
		}
	}
}

func (e *Core) execute(ctx context.Context, executable *entity.ExecutableBlock) error {
	if !e.stopControl.ShouldExecuteBlock(executable.Block.Header) {
		return nil
	}

	e.log.Info().
		Hex("block_id", logging.Entity(executable)).
		Uint64("height", executable.Block.Header.Height).
		Int("collections", len(executable.CompleteCollections)).
		Msgf("executing block")

	startedAt := time.Now()

	result, err := e.executor.ExecuteBlock(ctx, executable)
	if err != nil {
		return fmt.Errorf("failed to execute block %v: %w", executable.Block.ID(), err)
	}

	err = e.onBlockExecuted(ctx, executable, result, startedAt)
	if err != nil {
		return fmt.Errorf("failed to handle execution result of block %v: %w", executable.Block.ID(), err)
	}

	return nil
}

func (e *Core) fetch(missingColls []*block_queue.MissingCollection) error {
	missingCount := 0
	for _, col := range missingColls {

		// if we've requested this collection, we will store it in the storage,
		// so check the storage to see whether we've seen it.
		collection, err := e.collections.ByID(col.Guarantee.CollectionID)

		if err == nil {
			// we found the collection from storage, forward this collection to handler
			err = e.handleCollection(col.Guarantee.CollectionID, collection)
			if err != nil {
				return fmt.Errorf("could not handle collection: %w", err)
			}

			continue
		}

		// check if there was exception
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("error while querying for collection: %w", err)
		}

		err = e.collectionFetcher.FetchCollection(col.BlockID, col.Height, col.Guarantee)
		if err != nil {
			return fmt.Errorf("failed to fetch collection %v for block %v (height: %v): %w",
				col.Guarantee.ID(), col.BlockID, col.Height, err)
		}
		missingCount++
	}

	if missingCount > 0 {
		e.collectionFetcher.Force()
	}

	return nil
}
