// Package ingestion2 implements a modular ingestion engine responsible for
// orchestrating the processing of finalized blockchain data and receiving
// execution receipts from the network.
//
// The Engine coordinates several internal workers, each dedicated to a specific task:
//   - Receiving and persisting execution receipts from the network.
//   - Subscribing to finalized block events.
//   - Synchronizing collections associated with finalized blocks.
package ingestion2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/queue"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// defaultQueueCapacity is a capacity for the execution receipt message queue
	defaultQueueCapacity = 10_000

	// finalizedBlockProcessorWorkerCount defines the number of workers that
	// concurrently process finalized blocks in the job queue.
	// MUST be 1 to ensure sequential processing
	finalizedBlockProcessorWorkerCount = 1

	// searchAhead is a number of blocks that should be processed ahead by jobqueue
	// MUST be 1 to ensure sequential processing
	searchAhead = 1

	forestLoadInterval = time.Second * 10
)

// PipelineTask is a function that executes a pipeline task within the engine's worker pool.
// No errors are expected during normal operations.
type PipelineTask func(context.Context) error

type Engine struct {
	*component.ComponentManager

	log zerolog.Logger

	state            protocol.State
	blocks           storage.Blocks
	headers          storage.Headers
	receipts         storage.ExecutionReceipts
	executionResults storage.ExecutionResults

	resultsForest     *ResultsForest
	forestManager     *ForestManager
	pipelineTaskQueue *queue.ConcurrentPriorityQueue[PipelineTask]

	finalizedBlocksConsumer *jobqueue.ComponentConsumer
	finalizedBlocksNotifier engine.Notifier

	incorporatedBlocksQueue    *fifoqueue.FifoQueue
	incorporatedBlocksNotifier engine.Notifier

	messageHandler         *engine.MessageHandler
	executionReceiptsQueue *engine.FifoMessageStore

	collectionSyncer         *CollectionSyncer
	collectionExecutedMetric module.CollectionExecutedMetric
}

var _ network.MessageProcessor = (*Engine)(nil)
var _ hotstuff.FinalizationConsumer = (*Engine)(nil)

func New(
	log zerolog.Logger,
	net network.EngineRegistry,
	state protocol.State,
	blocks storage.Blocks,
	headers storage.Headers,
	receipts storage.ExecutionReceipts,
	executionResults storage.ExecutionResults,
	resultsForest *ResultsForest,
	forestManager *ForestManager,
	collectionSyncer *CollectionSyncer,
	collectionExecutedMetric module.CollectionExecutedMetric,
	finalizedProcessedHeight storage.ConsumerProgressInitializer,
	pipelineWorkerCount int,
) (*Engine, error) {
	executionReceiptsRawQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create execution receipts queue: %w", err)
	}
	executionReceiptsQueue := &engine.FifoMessageStore{FifoQueue: executionReceiptsRawQueue}
	messageHandler := engine.NewMessageHandler(
		log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.ExecutionReceipt)
				return ok
			},
			Store: executionReceiptsQueue,
		},
	)

	incorporatedBlocksQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create incorporated blocks queue: %w", err)
	}

	reader := jobqueue.NewFinalizedBlockReader(state, blocks)
	finalizedBlock, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block header: %w", err)
	}

	e := &Engine{
		log:                        log.With().Str("engine", "ingestion2").Logger(),
		state:                      state,
		blocks:                     blocks,
		headers:                    headers,
		receipts:                   receipts,
		executionResults:           executionResults,
		resultsForest:              resultsForest,
		forestManager:              forestManager,
		collectionSyncer:           collectionSyncer,
		messageHandler:             messageHandler,
		executionReceiptsQueue:     executionReceiptsQueue,
		collectionExecutedMetric:   collectionExecutedMetric,
		pipelineTaskQueue:          queue.NewConcurrentPriorityQueue[PipelineTask](true),
		finalizedBlocksNotifier:    engine.NewNotifier(),
		incorporatedBlocksQueue:    incorporatedBlocksQueue,
		incorporatedBlocksNotifier: engine.NewNotifier(),
	}

	finalizedBlocksConsumer, err := jobqueue.NewComponentConsumer(
		log.With().Str("module", "ingestion_block_consumer").Logger(),
		e.finalizedBlocksNotifier.Channel(),
		finalizedProcessedHeight,
		reader,
		finalizedBlock.Height,
		e.processFinalizedBlockJobCallback,
		finalizedBlockProcessorWorkerCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating finalized block jobqueue: %w", err)
	}

	e.finalizedBlocksConsumer = finalizedBlocksConsumer

	// register our workers which are basically consumers of different kinds of data.
	// engine notifies workers when new data is available so that they can start processing them.
	builder := component.NewComponentManagerBuilder().
		AddWorker(e.messageHandlerLoop).
		AddWorker(e.finalizedBlockProcessorLoop).
		AddWorker(e.incorporatedBlocksProcessorLoop).
		AddWorker(e.collectionSyncer.StartWorkerLoop).
		AddWorker(e.forestFeederLoop)

	for range pipelineWorkerCount {
		builder.AddWorker(e.pipelineWorkerLoop)
	}

	e.ComponentManager = builder.Build()

	// engine gets execution receipts from channels.ReceiveReceipts channel
	_, err = net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine in network to receive execution receipts: %w", err)
	}

	return e, nil
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
//
// No errors are expected during normal operations.
func (e *Engine) Process(chanName channels.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-e.ComponentManager.ShutdownSignal():
		return component.ErrComponentShutdown
	default:
	}

	switch event.(type) {
	case *flow.ExecutionReceipt:
		err := e.messageHandler.Process(originID, event)
		return err
	default:
		return fmt.Errorf("got invalid event type (%T) from %s channel", event, chanName)
	}
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated.
// Receives block finalized events from the finalization distributor and forwards them to the consumer.
func (e *Engine) OnFinalizedBlock(_ *model.Block) {
	e.finalizedBlocksNotifier.Notify()
}

// OnBlockIncorporated is called by the follower engine after a block has been incorporated and the state has been updated.
// Receives block incorporated events from the finalization distributor and forwards them to the consumer.
func (e *Engine) OnBlockIncorporated(block *model.Block) {
	e.incorporatedBlocksQueue.Push(block)
	e.incorporatedBlocksNotifier.Notify()
}

// messageHandlerLoop reacts to message handler notifications and processes available execution receipts
// once notification has arrived.
func (e *Engine) messageHandlerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableExecutionReceipts(ctx)
			if err != nil {
				// if an error reaches this point, it is unexpected
				ctx.Throw(err)
				return
			}
		}
	}
}

// processAvailableExecutionReceipts processes available execution receipts in the queue and handles it.
// It continues processing until all enqueued receipts are handled or the context is canceled.
//
// No errors are expected during normal operations.
func (e *Engine) processAvailableExecutionReceipts(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		msg, ok := e.executionReceiptsQueue.Get()
		if !ok {
			return nil
		}

		receipt := msg.Payload.(*flow.ExecutionReceipt)
		if err := e.processExecutionReceipt(receipt); err != nil {
			return err
		}
	}
}

// processExecutionReceipt processes the execution receipt.
//
// No errors are expected during normal operations.
func (e *Engine) processExecutionReceipt(receipt *flow.ExecutionReceipt) error {
	// since execution nodes optimistically execute blocks, it is expected that we will occasionally
	// receive receipts for blocks that are not yet locally certified. In this case, we should ignore
	// the receipts to avoid a byzantine spamming attack from the execution nodes.
	// all valid receipts will eventually be included in a certified block at which time they will be
	// stored and indexed.
	_, err := e.headers.ByBlockID(receipt.BlockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get header for block %s: %w", receipt.BlockID, err)
	}

	// persist the execution receipt locally, storing will also index the result
	err = e.receipts.Store(receipt)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	e.collectionExecutedMetric.ExecutionReceiptReceived(receipt)

	// most likely the block is certified, but we could receive delayed receipts
	blockStatus, _, err := e.resolveBlockStatus(receipt.BlockID)
	if err != nil {
		return fmt.Errorf("failed to resolve block status for block %s: %w", receipt.BlockID, err)
	}

	if _, err := e.resultsForest.AddReceipt(receipt.Stub(), blockStatus); err != nil {
		if !errors.Is(err, ErrMaxViewDeltaExceeded) {
			return fmt.Errorf("failed to add receipt to results forest: %w", err)
		}
	}

	return nil
}

// incorporatedBlocksProcessorLoop is a component.ComponentWorker that continuously processes incorporated blocks.
func (e *Engine) incorporatedBlocksProcessorLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	ch := e.incorporatedBlocksNotifier.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if err := e.processAvailableIncorporatedBlocks(ctx); err != nil {
				ctx.Throw(fmt.Errorf("failed to process incorporated block: %w", err))
				return
			}
		}
	}
}

// processAvailableIncorporatedBlocks processes all incorporated blocks available in the queue.
// No errors are expected during normal operations.
func (e *Engine) processAvailableIncorporatedBlocks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		block, ok := e.incorporatedBlocksQueue.Pop()
		if !ok {
			return nil
		}

		if err := e.processIncorporatedBlock(ctx, block.(*model.Block)); err != nil {
			return err
		}
	}
}

// processIncorporatedBlock processes an incorporated block.
// No errors are expected during normal operations.
func (e *Engine) processIncorporatedBlock(ctx context.Context, hotstuffBlock *model.Block) error {
	block, err := e.blocks.ByID(hotstuffBlock.BlockID)
	if err != nil {
		return fmt.Errorf("failed to get block from blocks storage: %w", err)
	}

	// extract all execution receipts from the block, and add them into the forest
	_, err = e.resultsForest.AddReceipts(block.Payload.Receipts, BlockStatusCertified)
	if err != nil {
		if errors.Is(err, ErrMaxViewDeltaExceeded) {
			// stop processing receipts after the forest rejects the receipt
			return nil
		}
		return fmt.Errorf("failed to add receipt to results forest: %w", err)
	}
	return nil
}

// finalizedBlockProcessorLoop runs the finalized blocks jobqueue consumer.
func (e *Engine) finalizedBlockProcessorLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.finalizedBlocksConsumer.Start(ctx)

	err := util.WaitClosed(ctx, e.finalizedBlocksConsumer.Ready())
	if err == nil {
		ready()
	}

	<-e.finalizedBlocksConsumer.Done()
}

// processFinalizedBlockJobCallback is a jobqueue callback that processes a finalized block job.
func (e *Engine) processFinalizedBlockJobCallback(
	ctx irrecoverable.SignalerContext,
	job module.Job,
	done func(),
) {
	block, err := jobqueue.JobToBlock(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to block: %w", err))
		return
	}

	err = e.indexFinalizedBlock(block)
	if err != nil {
		e.log.Error().Err(err).
			Str("job_id", string(job.ID())).
			Msg("unexpected error during finalized block processing job")
		ctx.Throw(fmt.Errorf("failed to index finalized block: %w", err))
		return
	}

	done()
}

// indexFinalizedBlock indexes the given finalized block’s collection guarantees and execution results,
// and requests related collections from the syncer.
//
// No errors are expected during normal operations.
func (e *Engine) indexFinalizedBlock(block *flow.Block) error {
	err := e.blocks.IndexBlockForCollectionGuarantees(block.ID(), flow.GetIDs(block.Payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not index block for collections: %w", err)
	}

	// loop through seals and index ID -> result ID
	for _, seal := range block.Payload.Seals {
		err := e.executionResults.Index(seal.BlockID, seal.ResultID)
		if err != nil {
			return fmt.Errorf("could not index block for execution result: %w", err)
		}
	}

	e.collectionSyncer.RequestCollectionsForBlock(block.Height, block.Payload.Guarantees)
	e.collectionExecutedMetric.BlockFinalized(block)

	// OnBlockFinalized should be called on the results forest first to ensure that the forest state
	// is up-to-date.
	if err := e.resultsForest.OnBlockFinalized(block); err != nil {
		return fmt.Errorf("result forest failed to process finalized block: %w", err)
	}

	e.forestManager.OnBlockFinalized(block)

	return nil
}

// pipelineWorkerLoop is a component.ComponentWorker that continuously processes pipeline tasks.
func (e *Engine) pipelineWorkerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	ch := e.pipelineTaskQueue.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			if err := e.processAllPipelineTasks(ctx); err != nil {
				ctx.Throw(fmt.Errorf("failed to process pipeline tasks: %w", err))
				return
			}
		}
	}
}

// processAllPipelineTasks processes all pipeline tasks availablein the queue.
// It continues processing until all tasks are processed or the context is canceled.
//
// No errors are expected during normal operations.
func (e *Engine) processAllPipelineTasks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		execute, ok := e.pipelineTaskQueue.Pop()
		if !ok {
			return nil
		}
		if err := execute(ctx); err != nil {
			return err
		}
	}
}

// forestFeederLoop loads results into the results forest during startup and when backfilling is needed.
func (e *Engine) forestFeederLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ticker := time.NewTicker(forestLoadInterval)
	defer ticker.Stop()

	isInitialLoad := true
	ready()

mainLoop:
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		var startView uint64
		if isInitialLoad {
			// on initial load, start from the forest's last sealed view, and load all results from storage
			startView = e.resultsForest.LastSealedView()
		} else {
			var hasRejected bool
			startView, hasRejected = e.resultsForest.ResetLowestRejectedView()
			if !hasRejected {
				continue // nothing to do
			}
		}

		// starting from the forest's last sealed view, add all sealed results until the forest
		// rejects or we reach the latest sealed block. iterating in a loop to guarantee that the
		// process does not stop until the forest's last sealed view matches the latest sealed view
		// in protocol state.
		previousLastSealedView := startView
		for {
			lastSealedView, done, err := e.loadAllSealedResults(ctx, startView)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					ctx.Throw(fmt.Errorf("failed to load sealed results: %w", err))
				}
				return
			}
			if !done {
				// loading/backfilling is not complete, continue after a delay
				continue mainLoop
			}

			// repeat loading until it is fully caught up with the latest sealed block from
			// state. This ensures that we catch the case where sealing progressed during loading.
			if lastSealedView == previousLastSealedView {
				break
			}
			previousLastSealedView = lastSealedView
		}

		done, err := e.loadAllUnsealedResults(ctx, previousLastSealedView)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				ctx.Throw(fmt.Errorf("failed to load unsealed results: %w", err))
			}
			return
		}
		if !done {
			// loading/backfilling is not complete, continue after a delay
			continue mainLoop
		}

		isInitialLoad = false
	}
}

// loadAllSealedResults loads all sealed results into the forest.
// No errors are expected during normal operations.
func (e *Engine) loadAllSealedResults(ctx context.Context, startView uint64) (lastInsertedView uint64, isDone bool, err error) {
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get sealed block: %w", err)
	}

	if startView == sealed.View {
		return sealed.View, true, nil
	}

	startHeader, err := e.headers.ByView(startView)
	if err != nil {
		return 0, false, fmt.Errorf("failed to get sealed header by view %d: %w", startView, err)
	}

	height := startHeader.Height
	for height <= sealed.Height {
		if ctx.Err() != nil {
			return 0, false, ctx.Err()
		}

		header, err := e.headers.ByHeight(height)
		if err != nil {
			return 0, false, fmt.Errorf("failed to get block ID by height for sealed block %d: %w", sealed.Height, err)
		}

		blockID := header.ID()
		executionResult, err := e.executionResults.ByBlockID(blockID)
		if err != nil {
			return 0, false, fmt.Errorf("failed to get execution result for sealed block %s: %w", blockID, err)
		}

		err = e.resultsForest.AddSealedResult(executionResult)
		if err != nil {
			if errors.Is(err, ErrMaxViewDeltaExceeded) {
				// forest is full, stop loading for now and continue later
				return header.ParentView, false, nil
			}
			return 0, false, fmt.Errorf("failed to add sealed result to forest: %w", err)
		}

		height++
	}
	return sealed.View, true, nil
}

// loadAllUnsealedResults loads all unsealed results with protocol state into the forest.
// No errors are expected during normal operations.
func (e *Engine) loadAllUnsealedResults(ctx context.Context, startView uint64) (isDone bool, err error) {
	startHeader, err := e.headers.ByView(startView)
	if err != nil {
		return false, fmt.Errorf("failed to get header by view %d: %w", startView, err)
	}
	startBlockID := startHeader.ID()

	descendants, err := e.state.AtBlockID(startBlockID).Descendants()
	if err != nil {
		return false, fmt.Errorf("failed to get descendants for certified block %s: %w", startBlockID, err)
	}

	for _, blockID := range descendants {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		blockStatus, err := e.resolveBlockStatus(blockID)
		if err != nil {
			return false, fmt.Errorf("failed to resolve block status for block %s: %w", blockID, err)
		}

		receipts, err := e.receipts.ByBlockID(blockID)
		if err != nil {
			return false, fmt.Errorf("failed to get receipts for block %s: %w", blockID, err)
		}

		_, err = e.resultsForest.AddReceipts(receipts.Stubs(), blockStatus)
		if err != nil {
			if errors.Is(err, ErrMaxViewDeltaExceeded) {
				// forest is full, stop loading for now and continue later
				return false, nil
			}
			return false, fmt.Errorf("failed to add unsealed result to forest: %w", err)
		}
	}

	return true, nil
}

// resolveBlockStatus resolves the block status for the given block ID.
// No errors are expected during normal operations.
func (e *Engine) resolveBlockStatus(blockID flow.Identifier) (status BlockStatus, err error) {
	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return 0, fmt.Errorf("failed to get header for block %s: %w", blockID, err)
	}

	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return 0, fmt.Errorf("failed to get sealed block: %w", err)
	}

	if header.View <= sealed.View {
		return BlockStatusSealed, nil
	}

	// the headers by height index is only populated for finalized blocks. further, consensus
	// guarantees that only a single block can be certified at a given view. we can use these to
	// determine if the block is finalized by checking if the block returned from headers.ByHeight
	// has the same view as the block in question. this allows us to avoid calculating the ID.
	headerByHeight, err := e.headers.ByHeight(header.Height)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// there is no finalized block at this height
			return BlockStatusCertified, nil
		}
		return 0, fmt.Errorf("failed to get header by height for block %s: %w", blockID, err)
	}

	if headerByHeight.View != header.View {
		// this block conflicts with the finalized block at this height
		return BlockStatusCertified, nil
	}

	return BlockStatusFinalized, nil
}
