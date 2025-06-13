package ingestion2

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// time to wait for the all the missing collections to be received at node startup
	collectionCatchupTimeout = 30 * time.Second

	// time to poll the storage to check if missing collections have been received
	collectionCatchupDBPollInterval = 10 * time.Millisecond

	// time to request missing collections from the network
	missingCollsRequestInterval = 1 * time.Minute

	// a threshold of number of blocks with missing collections beyond which collections should be re-requested
	// this is to prevent spamming the collection nodes with request
	missingCollsForBlockThreshold = 100

	// a threshold of block height beyond which collections should be re-requested (regardless of the number of blocks for which collection are missing)
	// this is to ensure that if a collection is missing for a long time (in terms of block height) it is eventually re-requested
	missingCollsForAgeThreshold = 100

	// time to update the FullBlockHeight index
	fullBlockRefreshInterval = 1 * time.Second

	defaultQueueCapacity = 10_000

	// processFinalizedBlocksWorkersCount defines the number of workers that
	// concurrently process finalized blocks in the job queue.
	processFinalizedBlocksWorkersCount = 1

	// searchAhead is a number of blocks that should be processed ahead by jobqueue
	searchAhead = 1
)

type Engine struct {
	*component.ComponentManager

	log                      zerolog.Logger
	collectionExecutedMetric module.CollectionExecutedMetric

	messageHandler            *engine.MessageHandler
	executionReceiptsNotifier engine.Notifier
	executionReceiptsQueue    engine.MessageStore
	executionReceipts         storage.ExecutionReceipts
	executionResults          storage.ExecutionResults

	finalizedBlockConsumer *jobqueue.ComponentConsumer
	finalizedBlockNotifier engine.Notifier
	blocks                 storage.Blocks

	errorMessageRequester ErrorMessageRequester

	collectionSyncer *CollectionSyncer
}

var _ network.MessageProcessor = (*Engine)(nil)

func New(
	log zerolog.Logger,
	net network.EngineRegistry,
	state protocol.State,
	blocks storage.Blocks,
	executionReceipts storage.ExecutionReceipts,
	executionResults storage.ExecutionResults,
	finalizedProcessedHeight storage.ConsumerProgressInitializer, //TODO: what does processed mean in this contex?
	errorMessageRequester ErrorMessageRequester,
	collectionSyncer *CollectionSyncer,
	collectionExecutedMetric module.CollectionExecutedMetric,
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

	e := &Engine{
		log:                       log.With().Str("engine", "ingestion2").Logger(),
		messageHandler:            messageHandler,
		executionReceiptsNotifier: engine.NewNotifier(),
		executionReceiptsQueue:    executionReceiptsQueue,
		finalizedBlockNotifier:    engine.NewNotifier(),
		blocks:                    blocks,
		executionReceipts:         executionReceipts,
		executionResults:          executionResults,
		collectionExecutedMetric:  collectionExecutedMetric,
		errorMessageRequester:     errorMessageRequester,
		collectionSyncer:          collectionSyncer,
	}

	// Set up component manager
	builder := component.NewComponentManagerBuilder().
		AddWorker(e.processExecutionReceipts).
		AddWorker(e.runFinalizedBlockConsumer).
		AddWorker(e.collectionSyncer.RequestCollections).
		AddWorker(e.errorMessageRequester.Request)

	e.ComponentManager = builder.Build()

	// Set up finalized block consumer
	finalizedBlockReader := jobqueue.NewFinalizedBlockReader(state, blocks)

	finalizedBlock, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not get finalized block header: %w", err)
	}

	// create a jobqueue that will process new available finalized block. The `finalizedBlockNotifier` is used to
	// signal new work, which is being triggered on the `processFinalizedBlockJob` handler.
	e.finalizedBlockConsumer, err = jobqueue.NewComponentConsumer(
		e.log.With().Str("module", "ingestion_block_consumer").Logger(),
		e.finalizedBlockNotifier.Channel(),
		finalizedProcessedHeight,
		finalizedBlockReader,
		finalizedBlock.Height,
		e.processFinalizedBlockJob,
		processFinalizedBlocksWorkersCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating finalizedBlock jobqueue: %w", err)
	}

	// engine gets execution receipts from channels.ReceiveReceipts channel
	_, err = net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	return e, nil
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
//
// TODO: I'm curious why we process exec receipts queue in 1 routine (blocking call) instead of multiple goroutines.
// Is it done to prevent some kind of attack where bigger node (EN) spams
// smaller AN engines with millions of messages?
func (e *Engine) Process(chanName channels.Channel, originID flow.Identifier, event interface{}) error {
	//TODO: explain why we need this check.
	select {
	case <-e.ComponentManager.ShutdownSignal():
		return component.ErrComponentShutdown
	default:
	}

	switch event.(type) {
	case *flow.ExecutionReceipt:
		err := e.messageHandler.Process(originID, event)
		e.executionReceiptsNotifier.Notify()
		return err
	default:
		return fmt.Errorf("got invalid event type (%T) from %s channel", event, chanName)
	}
}

// processExecutionReceipts is responsible for processing the execution receipts.
// It listens for incoming execution receipts and processes them asynchronously.
func (e *Engine) processExecutionReceipts(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.executionReceiptsNotifier.Channel():
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
// It continues processing until the context is canceled.
//
// No errors are expected during normal operation.
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
		if err := e.persistExecutionReceipt(receipt); err != nil {
			return err
		}

		// Notify to fetch and store transaction result error messages for the block
		e.errorMessageRequester.Notify(receipt.BlockID)
	}
}

// persistExecutionReceipt persists the execution receipt locally.
// Storing the execution receipt and updates the collection executed metric.
//
// No errors are expected during normal operation.
func (e *Engine) persistExecutionReceipt(receipt *flow.ExecutionReceipt) error {
	// persist the execution receipt locally, storing will also index the receipt
	err := e.executionReceipts.Store(receipt)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	e.collectionExecutedMetric.ExecutionReceiptReceived(receipt)
	return nil
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated.
// Receives block finalized events from the finalization distributor and forwards them to the finalizedBlockConsumer.
func (e *Engine) OnFinalizedBlock(_ *model.Block) {
	e.finalizedBlockNotifier.Notify()
}

// runFinalizedBlockConsumer runs the finalizedBlockConsumer component
func (e *Engine) runFinalizedBlockConsumer(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.finalizedBlockConsumer.Start(ctx)

	err := util.WaitClosed(ctx, e.finalizedBlockConsumer.Ready())
	if err == nil {
		ready()
	}

	<-e.finalizedBlockConsumer.Done()
}

// processFinalizedBlockJob is a handler function for processing finalized block jobs.
// It converts the job to a block, processes the block, and logs any errors encountered during processing.
func (e *Engine) processFinalizedBlockJob(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
	block, err := jobqueue.JobToBlock(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to block: %w", err))
	}

	err = e.processFinalizedBlock(block)
	if err == nil {
		done()
		return
	}

	e.log.Error().Err(err).Str("job_id", string(job.ID())).Msg("error during finalized block processing job")
}

// processFinalizedBlock handles an incoming finalized block.
// It processes the block, indexes it for further processing, and requests missing collections if necessary.
//
// Expected errors during normal operation:
//   - storage.ErrNotFound - if last full block height does not exist in the database.
//   - storage.ErrAlreadyExists - if the collection within block or an execution result ID already exists in the database.
//   - generic error in case of unexpected failure from the database layer, or failure
//     to decode an existing database value.
func (e *Engine) processFinalizedBlock(block *flow.Block) error {
	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// TODO: substitute an indexer module as layer between engine and storage

	// index the block storage with each of the collection guarantee
	err := e.blocks.IndexBlockForCollections(block.Header.ID(), flow.GetIDs(block.Payload.Guarantees))
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

	e.collectionSyncer.RequestCollectionsForBlock(block.Header.Height, block.Payload.Guarantees)
	e.collectionExecutedMetric.BlockFinalized(block)

	return nil
}
