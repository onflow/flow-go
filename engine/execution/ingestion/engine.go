package ingestion

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
	"github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/pruner"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	psEvents "github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	psEvents.Noop // satisfy protocol events consumer interface

	unit                *engine.Unit
	log                 zerolog.Logger
	me                  module.Local
	collectionFetcher   CollectionFetcher
	headers             storage.Headers // see comments on getHeaderByHeight for why we need it
	blocks              storage.Blocks
	collections         storage.Collections
	computationManager  computation.ComputationManager
	providerEngine      provider.ProviderEngine
	mempool             *Mempool
	execState           state.ExecutionState
	metrics             module.ExecutionMetrics
	maxCollectionHeight uint64
	tracer              module.Tracer
	extensiveLogging    bool
	executionDataPruner *pruner.Pruner
	uploader            *uploader.Manager
	stopControl         *stop.StopControl
	loader              BlockLoader
}

func New(
	unit *engine.Unit,
	logger zerolog.Logger,
	net network.EngineRegistry,
	me module.Local,
	collectionFetcher CollectionFetcher,
	headers storage.Headers,
	blocks storage.Blocks,
	collections storage.Collections,
	executionEngine computation.ComputationManager,
	providerEngine provider.ProviderEngine,
	execState state.ExecutionState,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	extLog bool,
	pruner *pruner.Pruner,
	uploader *uploader.Manager,
	stopControl *stop.StopControl,
	loader BlockLoader,
) (*Engine, error) {
	log := logger.With().Str("engine", "ingestion").Logger()

	mempool := newMempool()

	eng := Engine{
		unit:                unit,
		log:                 log,
		me:                  me,
		collectionFetcher:   collectionFetcher,
		headers:             headers,
		blocks:              blocks,
		collections:         collections,
		computationManager:  executionEngine,
		providerEngine:      providerEngine,
		mempool:             mempool,
		execState:           execState,
		metrics:             metrics,
		maxCollectionHeight: 0,
		tracer:              tracer,
		extensiveLogging:    extLog,
		executionDataPruner: pruner,
		uploader:            uploader,
		stopControl:         stopControl,
		loader:              loader,
	}

	return &eng, nil
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	if e.stopControl.IsExecutionStopped() {
		return e.unit.Ready()
	}

	if err := e.uploader.RetryUploads(); err != nil {
		e.log.Warn().Msg("failed to re-upload all ComputationResults")
	}

	err := e.reloadUnexecutedBlocks()
	if err != nil {
		e.log.Fatal().Err(err).Msg("failed to load all unexecuted blocks")
	}

	return e.unit.Ready()
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(
	channel channels.Channel,
	originID flow.Identifier,
	event interface{},
) {
	e.unit.Launch(func() {
		err := e.process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return fmt.Errorf("ingestion error does not process local events")
}

func (e *Engine) Process(
	channel channels.Channel,
	originID flow.Identifier,
	event interface{},
) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	return nil
}

// on nodes startup, we need to load all the unexecuted blocks to the execution queues.
// blocks have to be loaded in the way that the parent has been loaded before loading its children
func (e *Engine) reloadUnexecutedBlocks() error {
	unexecuted, err := e.loader.LoadUnexecuted(e.unit.Ctx())
	if err != nil {
		return fmt.Errorf("could not load unexecuted blocks: %w", err)
	}
	// it's possible the BlockProcessable is called during the reloading, as the follower engine
	// will receive blocks before ingestion engine is ready.
	// The problem with that is, since the reloading hasn't finished yet, enqueuing the new block from
	// the BlockProcessable callback will fail, because its parent block might have not been reloaded
	// to the queues yet.
	// So one solution here is to lock the execution queues during reloading, so that if BlockProcessable
	// is called before reloading is finished, it will be blocked, which will avoid that edge case.
	return e.mempool.Run(func(
		blockByCollection *stdmap.BlockByCollectionBackdata,
		executionQueues *stdmap.QueuesBackdata,
	) error {
		for _, blockID := range unexecuted {
			err := e.reloadBlock(blockByCollection, executionQueues, blockID)
			if err != nil {
				return fmt.Errorf("could not reload block: %v, %w", blockID, err)
			}

			e.log.Debug().Hex("block_id", blockID[:]).Msg("reloaded block")
		}

		e.log.Info().Int("count", len(unexecuted)).Msg("all unexecuted have been successfully reloaded")

		return nil
	})
}

func (e *Engine) reloadBlock(
	blockByCollection *stdmap.BlockByCollectionBackdata,
	executionQueues *stdmap.QueuesBackdata,
	blockID flow.Identifier,
) error {
	block, err := e.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("could not get block by ID: %v %w", blockID, err)
	}

	// enqueue the block and check if there is any missing collections
	missingCollections, err := e.enqueueBlockAndCheckExecutable(blockByCollection, executionQueues, block, false)

	if err != nil {
		return fmt.Errorf("could not enqueue block %x on reloading: %w", blockID, err)
	}

	// forward the missing collections to requester engine for requesting them from collection nodes,
	// adding the missing collections to mempool in order to trigger the block execution as soon as
	// all missing collections are received.
	err = e.fetchAndHandleCollection(blockID, block.Header.Height, missingCollections, func(collection *flow.Collection) error {
		err := e.addCollectionToMempool(collection, blockByCollection)

		if err != nil {
			return fmt.Errorf("could not add collection to mempool: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("could not fetch or handle collection %w", err)
	}
	return nil
}

// BlockProcessable handles the new verified blocks (blocks that
// have passed consensus validation) received from the consensus nodes
// NOTE: BlockProcessable might be called multiple times for the same block.
// NOTE: Ready calls reloadUnexecutedBlocks during initialization, which handles dropped protocol events.
func (e *Engine) BlockProcessable(b *flow.Header, _ *flow.QuorumCertificate) {

	// TODO: this should not be blocking: https://github.com/onflow/flow-go/issues/4400

	// skip if stopControl tells to skip
	if !e.stopControl.ShouldExecuteBlock(b) {
		return
	}

	blockID := b.ID()
	newBlock, err := e.blocks.ByID(blockID)
	if err != nil {
		e.log.Fatal().Err(err).Msgf("could not get incorporated block(%v): %v", blockID, err)
	}

	e.log.Info().Hex("block_id", blockID[:]).
		Uint64("height", b.Height).
		Msg("handling new block")

	err = e.handleBlock(e.unit.Ctx(), newBlock)
	if err != nil {
		e.log.Error().Err(err).Hex("block_id", blockID[:]).Msg("failed to handle block")
	}
}

// Main handling

// handle block will process the incoming block.
// the block has passed the consensus validation.
func (e *Engine) handleBlock(ctx context.Context, block *flow.Block) error {

	blockID := block.ID()
	log := e.log.With().Hex("block_id", blockID[:]).Logger()

	span, _ := e.tracer.StartBlockSpan(ctx, blockID, trace.EXEHandleBlock)
	defer span.End()

	executed, err := e.execState.IsBlockExecuted(block.Header.Height, blockID)
	if err != nil {
		return fmt.Errorf("could not check whether block is executed: %w", err)
	}

	if executed {
		log.Debug().Msg("block has been executed already")
		return nil
	}

	var missingCollections []*flow.CollectionGuarantee
	// unexecuted block
	// acquiring the lock so that there is only one process modifying the queue
	err = e.mempool.Run(func(
		blockByCollection *stdmap.BlockByCollectionBackdata,
		executionQueues *stdmap.QueuesBackdata,
	) error {
		missing, err := e.enqueueBlockAndCheckExecutable(blockByCollection, executionQueues, block, false)
		if err != nil {
			return err
		}
		missingCollections = missing
		return nil
	})

	if err != nil {
		return fmt.Errorf("could not enqueue block %v: %w", blockID, err)
	}

	return e.addOrFetch(blockID, block.Header.Height, missingCollections)
}

func (e *Engine) enqueueBlockAndCheckExecutable(
	blockByCollection *stdmap.BlockByCollectionBackdata,
	executionQueues *stdmap.QueuesBackdata,
	block *flow.Block,
	checkStateSync bool,
) ([]*flow.CollectionGuarantee, error) {
	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*entity.CompleteCollection),
	}

	blockID := executableBlock.ID()

	lg := e.log.With().
		Hex("block_id", blockID[:]).
		Uint64("block_height", executableBlock.Block.Header.Height).
		Logger()

	// adding the block to the queue,
	queue, added, head := enqueue(executableBlock, executionQueues)

	// if it's not added, it means the block is not a new block, it already
	// exists in the queue, then bail
	if !added {
		log.Debug().Hex("block_id", logging.Entity(executableBlock)).
			Int("block_height", int(executableBlock.Height())).
			Msg("block already exists in the execution queue")
		return nil, nil
	}

	firstUnexecutedHeight := queue.Head.Item.Height()

	// check if a block is executable.
	// a block is executable if the following conditions are all true
	// 1) the parent state commitment is ready
	// 2) the collections for the block payload are ready
	// 3) the child block is ready for querying the randomness

	// check if the block's parent has been executed. (we can't execute the block if the parent has
	// not been executed yet)
	// check if there is a statecommitment for the parent block
	parentCommitment, err := e.execState.StateCommitmentByBlockID(block.Header.ParentID)

	// if we found the statecommitment for the parent block, then add it to the executable block.
	if err == nil {
		executableBlock.StartState = &parentCommitment
	} else if errors.Is(err, storage.ErrNotFound) {
		// the parent block is an unexecuted block.
		// if the queue only has one block, and its parent doesn't
		// exist in the queue, then we need to load the block from the storage.
		_, ok := queue.Nodes[blockID]
		if !ok {
			lg.Error().Msgf("an unexecuted parent block is missing in the queue")
		}
	} else {
		// if there is exception, then crash
		lg.Fatal().Err(err).Msg("unexpected error while accessing storage, shutting down")
	}

	// check if we have all the collections for the block, and request them if there is missing.
	missingCollections, err := e.matchAndFindMissingCollections(executableBlock, blockByCollection)
	if err != nil {
		return nil, fmt.Errorf("cannot send collection requests: %w", err)
	}

	complete := false

	// if newly enqueued block is inside any existing queue, we should skip now and wait
	// for parent to finish execution
	if head {
		// execute the block if the block is ready to be executed
		complete = e.executeBlockIfComplete(executableBlock)
	}

	lg.Info().
		// if the execution is halt, but the queue keeps growing, we could check which block
		// hasn't been executed.
		Uint64("first_unexecuted_in_queue", firstUnexecutedHeight).
		Bool("complete", complete).
		Bool("head_of_queue", head).
		Int("cols", len(executableBlock.Block.Payload.Guarantees)).
		Int("missing_cols", len(missingCollections)).
		Msg("block is enqueued")

	return missingCollections, nil
}

// executeBlock will execute the block.
// When finish executing, it will check if the children becomes executable and execute them if yes.
func (e *Engine) executeBlock(
	ctx context.Context,
	executableBlock *entity.ExecutableBlock,
) {
	lg := e.log.With().
		Hex("block_id", logging.Entity(executableBlock)).
		Uint64("height", executableBlock.Block.Header.Height).
		Int("collections", len(executableBlock.CompleteCollections)).
		Logger()

	lg.Info().Msg("executing block")

	startedAt := time.Now()

	span, ctx := e.tracer.StartSpanFromContext(ctx, trace.EXEExecuteBlock)
	defer span.End()

	parentID := executableBlock.Block.Header.ParentID
	parentErID, err := e.execState.GetExecutionResultID(ctx, parentID)
	if err != nil {
		lg.Err(err).
			Str("parentID", parentID.String()).
			Msg("could not get execution result ID for parent block")
		return
	}

	snapshot := e.execState.NewStorageSnapshot(*executableBlock.StartState,
		executableBlock.Block.Header.ParentID,
		executableBlock.Block.Header.Height-1,
	)

	computationResult, err := e.computationManager.ComputeBlock(
		ctx,
		parentErID,
		executableBlock,
		snapshot)
	if err != nil {
		lg.Err(err).Msg("error while computing block")
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()

	go func() {
		defer wg.Done()
		err := e.uploader.Upload(ctx, computationResult)
		if err != nil {
			lg.Err(err).Msg("error while uploading block")
			// continue processing. uploads should not block execution
		}
	}()

	err = e.saveExecutionResults(ctx, computationResult)
	if errors.Is(err, storage.ErrDataMismatch) {
		lg.Fatal().Err(err).Msg("fatal: trying to store different results for the same block")
	}

	if err != nil {
		lg.Err(err).Msg("error while handing computation results")
		return
	}

	receipt := computationResult.ExecutionReceipt
	broadcasted, err := e.providerEngine.BroadcastExecutionReceipt(
		ctx, executableBlock.Block.Header.Height, receipt)
	if err != nil {
		lg.Err(err).Msg("critical: failed to broadcast the receipt")
	}

	finalEndState := computationResult.CurrentEndState()
	lg.Info().
		Hex("parent_block", executableBlock.Block.Header.ParentID[:]).
		Int("collections", len(executableBlock.Block.Payload.Guarantees)).
		Hex("start_state", executableBlock.StartState[:]).
		Hex("final_state", finalEndState[:]).
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("result_id", logging.Entity(receipt.ExecutionResult)).
		Hex("execution_data_id", receipt.ExecutionResult.ExecutionDataID[:]).
		Bool("state_changed", finalEndState != *executableBlock.StartState).
		Uint64("num_txs", nonSystemTransactionCount(receipt.ExecutionResult)).
		Bool("broadcasted", broadcasted).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("block executed")

	err = e.onBlockExecuted(executableBlock, finalEndState)
	if err != nil {
		lg.Err(err).Msg("failed in process block's children")
	}

	if e.executionDataPruner != nil {
		e.executionDataPruner.NotifyFulfilledHeight(executableBlock.Height())
	}

	e.stopControl.OnBlockExecuted(executableBlock.Block.Header)

	e.unit.Ctx()

}

func nonSystemTransactionCount(result flow.ExecutionResult) uint64 {
	count := uint64(0)
	for _, chunk := range result.Chunks {
		count += chunk.NumberOfTransactions
	}
	return count
}

// we've executed the block, now we need to check:
// 1. whether the state syncing can be turned off
// 2. whether its children can be executed
//   the executionQueues stores blocks as a tree:
//
//   10 <- 11 <- 12
//   	 ^-- 13
//   14 <- 15 <- 16
//
//   if block 10 is the one just executed, then we will remove it from the queue, and add
//   its children back, meaning the tree will become:
//
//   11 <- 12
//   13
//   14 <- 15 <- 16

func (e *Engine) onBlockExecuted(
	executed *entity.ExecutableBlock,
	finalState flow.StateCommitment,
) error {

	e.metrics.ExecutionStorageStateCommitment(int64(len(finalState)))
	e.metrics.ExecutionLastExecutedBlockHeight(executed.Block.Header.Height)

	missingCollections := make(map[*entity.ExecutableBlock][]*flow.CollectionGuarantee)
	err := e.mempool.Run(
		func(
			blockByCollection *stdmap.BlockByCollectionBackdata,
			executionQueues *stdmap.QueuesBackdata,
		) error {
			// find the block that was just executed
			executionQueue, exists := executionQueues.ByID(executed.ID())
			if !exists {
				logQueueState(e.log, executionQueues, executed.ID())
				// when the block no longer exists in the queue, it means there was a race condition that
				// two onBlockExecuted was called for the same block, and one process has already removed the
				// block from the queue, so we will print an error here
				return fmt.Errorf("block has been executed already, no longer exists in the queue")
			}

			// dismount the executed block and all its children
			_, newQueues := executionQueue.Dismount()

			// go through each children, add them back to the queue, and check
			// if the children is executable
			for _, queue := range newQueues {
				queueID := queue.ID()
				added := executionQueues.Add(queueID, queue)
				if !added {
					// blocks should be unique in execution queues, if we dismount all the children blocks, then
					// add it back to the queues, then it should always be able to add.
					// If not, then there is a bug that the queues have duplicated blocks
					return fmt.Errorf("fatal error - child block already in execution queue")
				}

				// the parent block has been executed, update the StartState of
				// each child block.
				child := queue.Head.Item.(*entity.ExecutableBlock)
				child.StartState = &finalState

				missing, err := e.matchAndFindMissingCollections(child, blockByCollection)
				if err != nil {
					return fmt.Errorf("cannot send collection requests: %w", err)
				}
				if len(missing) > 0 {
					missingCollections[child] = append(missingCollections[child], missing...)
				}

				completed := e.executeBlockIfComplete(child)
				if !completed {
					e.log.Debug().
						Hex("executed_block", logging.Entity(executed)).
						Hex("child_block", logging.Entity(child)).
						Msg("child block is not ready to be executed yet")
				} else {
					e.log.Debug().
						Hex("executed_block", logging.Entity(executed)).
						Hex("child_block", logging.Entity(child)).
						Msg("child block is ready to be executed")
				}
			}

			// remove the executed block
			executionQueues.Remove(executed.ID())

			return nil
		})

	if err != nil {
		e.log.Fatal().Err(err).
			Hex("block", logging.Entity(executed)).
			Uint64("height", executed.Block.Header.Height).
			Msg("error while requeueing blocks after execution")
	}

	for child, missing := range missingCollections {
		err := e.addOrFetch(child.ID(), child.Block.Header.Height, missing)
		if err != nil {
			return fmt.Errorf("fail to add missing collections: %w", err)
		}
	}

	return nil
}

// executeBlockIfComplete checks whether the block is ready to be executed.
// if yes, execute the block
// return a bool indicates whether the block was completed
func (e *Engine) executeBlockIfComplete(eb *entity.ExecutableBlock) bool {

	if eb.Executing {
		return false
	}

	// if don't have the delta, then check if everything is ready for executing
	// the block
	if eb.IsComplete() {

		if e.extensiveLogging {
			e.logExecutableBlock(eb)
		}

		// no external synchronisation is used because this method must be run in a thread-safe context
		eb.Executing = true

		e.unit.Launch(func() {
			e.executeBlock(e.unit.Ctx(), eb)
		})
		return true
	}
	return false
}

// OnCollection is a callback for handling the collections requested by the
// collection requester.
func (e *Engine) OnCollection(originID flow.Identifier, entity flow.Entity) {
	// convert entity to strongly typed collection
	collection, ok := entity.(*flow.Collection)
	if !ok {
		e.log.Error().Msgf("invalid entity type (%T)", entity)
		return
	}

	// no need to validate the origin ID, since the collection requester has
	// checked the origin must be a collection node.

	err := e.handleCollection(originID, collection)
	if err != nil {
		e.log.Error().Err(err).Msg("could not handle collection")
	}
}

// a block can't be executed if its collection is missing.
// since a collection can belong to multiple blocks, we need to
// find all the blocks that are needing this collection, and then
// check if any of these block becomes executable and execute it if
// is.
func (e *Engine) handleCollection(
	originID flow.Identifier,
	collection *flow.Collection,
) error {
	collID := collection.ID()

	span, _ := e.tracer.StartCollectionSpan(context.Background(), collID, trace.EXEHandleCollection)
	defer span.End()

	lg := e.log.With().Hex("collection_id", collID[:]).Logger()

	lg.Info().Hex("sender", originID[:]).Int("len", collection.Len()).Msg("handle collection")
	defer func(startTime time.Time) {
		lg.Info().TimeDiff("duration", time.Now(), startTime).Msg("collection handled")
	}(time.Now())

	// TODO: bail if have seen this collection before.
	err := e.collections.Store(collection)
	if err != nil {
		return fmt.Errorf("cannot store collection: %w", err)
	}

	return e.mempool.BlockByCollection.Run(
		func(backdata *stdmap.BlockByCollectionBackdata) error {
			return e.addCollectionToMempool(collection, backdata)
		},
	)
}

func (e *Engine) addCollectionToMempool(
	collection *flow.Collection,
	backdata *stdmap.BlockByCollectionBackdata,
) error {
	collID := collection.ID()
	blockByCollectionID, exists := backdata.ByID(collID)

	// if we don't find any block for this collection, then
	// means we don't need this collection any more.
	// or it was ejected from the mempool when it was full.
	// either way, we will return
	if !exists {
		return nil
	}

	for _, executableBlock := range blockByCollectionID.ExecutableBlocks {
		blockID := executableBlock.ID()

		completeCollection, ok := executableBlock.CompleteCollections[collID]
		if !ok {
			return fmt.Errorf("cannot handle collection: internal inconsistency - collection pointing to block %v which does not contain said collection",
				blockID)
		}

		// record collection max height metrics
		blockHeight := executableBlock.Block.Header.Height
		if blockHeight > e.maxCollectionHeight {
			e.metrics.UpdateCollectionMaxHeight(blockHeight)
			e.maxCollectionHeight = blockHeight
		}

		if completeCollection.IsCompleted() {
			// already received transactions for this collection
			continue
		}

		// update the transactions of the collection
		// Note: it's guaranteed the transactions are for this collection, because
		// the collection id matches with the CollectionID from the collection guarantee
		completeCollection.Transactions = collection.Transactions

		// check if the block becomes executable
		_ = e.executeBlockIfComplete(executableBlock)
	}

	// since we've received this collection, remove it from the index
	// this also prevents from executing the same block twice, because the second
	// time when the collection arrives, it will not be found in the blockByCollectionID
	// index.
	backdata.Remove(collID)

	return nil
}

func newQueue(blockify queue.Blockify, queues *stdmap.QueuesBackdata) (
	*queue.Queue,
	bool,
) {
	q := queue.NewQueue(blockify)
	qID := q.ID()
	return q, queues.Add(qID, q)
}

// enqueue adds a block to the queues, return the queue that includes the block and booleans
// * is block new one (it's not already enqueued, not a duplicate)
// * is head of the queue (new queue has been created)
//
// Queues are chained blocks. Since a block can't be executable until its parent has been
// executed, the chained structure allows us to only check the head of each queue to see if
// any block becomes executable.
// for instance we have one queue whose head is A:
//
//	A <- B <- C
//	  ^- D <- E
//
// If we receive E <- F, then we will add it to the queue:
//
//	A <- B <- C
//	  ^- D <- E <- F
//
// Even through there are 6 blocks, we only need to check if block A becomes executable.
// when the parent block isn't in the queue, we add it as a new queue. for instance, if
// we receive H <- G, then the queues will become:
//
//	A <- B <- C
//	  ^- D <- E
//	G
func enqueue(blockify queue.Blockify, queues *stdmap.QueuesBackdata) (
	*queue.Queue,
	bool,
	bool,
) {
	for _, queue := range queues.All() {
		if stored, isNew := queue.TryAdd(blockify); stored {
			return queue, isNew, false
		}
	}
	queue, isNew := newQueue(blockify, queues)
	return queue, isNew, true
}

// check if the block's collections have been received,
// if yes, add the collection to the executable block
// if no, fetch the collection.
// if a block has 3 collection, it would be 3 reqs to fetch them.
// mark the collection belongs to the block,
// mark the block contains this collection.
// It returns the missing collections to be fetched
// TODO: to rename
func (e *Engine) matchAndFindMissingCollections(
	executableBlock *entity.ExecutableBlock,
	collectionsBackdata *stdmap.BlockByCollectionBackdata,
) ([]*flow.CollectionGuarantee, error) {
	missingCollections := make([]*flow.CollectionGuarantee, 0, len(executableBlock.Block.Payload.Guarantees))

	for _, guarantee := range executableBlock.Block.Payload.Guarantees {
		coll := &entity.CompleteCollection{
			Guarantee: guarantee,
		}
		executableBlock.CompleteCollections[guarantee.ID()] = coll

		// check if we have requested this collection before.
		// blocksNeedingCollection stores all the blocks that contain this collection

		if blocksNeedingCollection, exists := collectionsBackdata.ByID(guarantee.ID()); exists {
			// if we've requested this collection, it means other block might also contain this collection.
			// in this case, add this block to the map so that when the collection is received,
			// we could update the executable block
			blocksNeedingCollection.ExecutableBlocks[executableBlock.ID()] = executableBlock

			// since the collection is still being requested, we don't have the transactions
			// yet, so exit
			continue
		}

		// the storage doesn't have this collection, meaning this is our first time seeing this
		// collection guarantee, create an entry to store in collectionsBackdata in order to
		// update the executable blocks when the collection is received.
		blocksNeedingCollection := &entity.BlocksByCollection{
			CollectionID:     guarantee.ID(),
			ExecutableBlocks: map[flow.Identifier]*entity.ExecutableBlock{executableBlock.ID(): executableBlock},
		}

		added := collectionsBackdata.Add(blocksNeedingCollection.ID(), blocksNeedingCollection)
		if !added {
			// sanity check, should not happen, unless mempool implementation has a bug
			return nil, fmt.Errorf("collection already mapped to block")
		}

		missingCollections = append(missingCollections, guarantee)
	}

	return missingCollections, nil
}

// save the execution result of a block
func (e *Engine) saveExecutionResults(
	ctx context.Context,
	result *execution.ComputationResult,
) error {
	span, childCtx := e.tracer.StartSpanFromContext(ctx, trace.EXESaveExecutionResults)
	defer span.End()

	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock)).
		Msg("received computation result")

	for _, event := range result.ExecutionResult.ServiceEvents {
		e.log.Info().
			Uint64("block_height", result.ExecutableBlock.Height()).
			Hex("block_id", logging.Entity(result.ExecutableBlock)).
			Str("event_type", event.Type.String()).
			Msg("service event emitted")
	}

	err := e.execState.SaveExecutionResults(childCtx, result)
	if err != nil {
		return fmt.Errorf("cannot persist execution state: %w", err)
	}

	finalEndState := result.CurrentEndState()
	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock)).
		Hex("start_state", result.ExecutableBlock.StartState[:]).
		Hex("final_state", finalEndState[:]).
		Msg("saved computation results")

	return nil
}

// logExecutableBlock logs all data about an executable block
// over time we should skip this
func (e *Engine) logExecutableBlock(eb *entity.ExecutableBlock) {
	// log block
	e.log.Debug().
		Hex("block_id", logging.Entity(eb)).
		Hex("prev_block_id", logging.ID(eb.Block.Header.ParentID)).
		Uint64("block_height", eb.Block.Header.Height).
		Int("number_of_collections", len(eb.Collections())).
		RawJSON("block_header", logging.AsJSON(eb.Block.Header)).
		Msg("extensive log: block header")

	// logs transactions
	for i, col := range eb.Collections() {
		for j, tx := range col.Transactions {
			e.log.Debug().
				Hex("block_id", logging.Entity(eb)).
				Int("block_height", int(eb.Block.Header.Height)).
				Hex("prev_block_id", logging.ID(eb.Block.Header.ParentID)).
				Int("collection_index", i).
				Int("tx_index", j).
				Hex("collection_id", logging.ID(col.Guarantee.CollectionID)).
				Hex("tx_hash", logging.Entity(tx)).
				Hex("start_state_commitment", eb.StartState[:]).
				RawJSON("transaction", logging.AsJSON(tx)).
				Msg("extensive log: executed tx content")
		}
	}
}

// addOrFetch checks if there are stored collections for the given guarantees, if there is,
// forward them to mempool to process the collection, otherwise fetch the collections.
// any error returned are exception
func (e *Engine) addOrFetch(
	blockID flow.Identifier,
	height uint64,
	guarantees []*flow.CollectionGuarantee,
) error {
	return e.fetchAndHandleCollection(blockID, height, guarantees, func(collection *flow.Collection) error {
		err := e.mempool.BlockByCollection.Run(
			func(backdata *stdmap.BlockByCollectionBackdata) error {
				return e.addCollectionToMempool(collection, backdata)
			})

		if err != nil {
			return fmt.Errorf("could not add collection to mempool: %w", err)
		}
		return nil
	})
}

// addOrFetch checks if there are stored collections for the given guarantees, if there is,
// forward them to the handler to process the collection, otherwise fetch the collections.
// any error returned are exception
func (e *Engine) fetchAndHandleCollection(
	blockID flow.Identifier,
	height uint64,
	guarantees []*flow.CollectionGuarantee,
	handleCollection func(*flow.Collection) error,
) error {
	fetched := false
	for _, guarantee := range guarantees {
		// if we've requested this collection, we will store it in the storage,
		// so check the storage to see whether we've seen it.
		collection, err := e.collections.ByID(guarantee.CollectionID)

		if err == nil {
			// we found the collection from storage, forward this collection to handler
			err = handleCollection(collection)
			if err != nil {
				return fmt.Errorf("could not handle collection: %w", err)
			}

			continue
		}

		// check if there was exception
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("error while querying for collection: %w", err)
		}

		err = e.collectionFetcher.FetchCollection(blockID, height, guarantee)
		if err != nil {
			return fmt.Errorf("could not fetch collection: %w", err)
		}
		fetched = true
	}

	// make sure that the requests are dispatched immediately by the requester
	if fetched {
		e.collectionFetcher.Force()
		e.metrics.ExecutionCollectionRequestSent()
	}

	return nil
}

func logQueueState(log zerolog.Logger, queues *stdmap.QueuesBackdata, blockID flow.Identifier) {
	all := queues.All()

	log.With().Hex("queue_state__executed_block_id", blockID[:]).Int("count", len(all)).Logger()
	for i, queue := range all {
		log.Error().Msgf("%v-th queue state: %v", i, queue.String())
	}
}
