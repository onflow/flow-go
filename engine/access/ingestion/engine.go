package ingestion

import (
	"context"
	"fmt"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
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

	// time to update the FullBlockHeight index
	fullBlockRefreshInterval = 1 * time.Second

	// time to request missing collections from the network
	missingCollsRequestInterval = 1 * time.Minute

	// a threshold of number of blocks with missing collections beyond which collections should be re-requested
	// this is to prevent spamming the collection nodes with request
	missingCollsForBlockThreshold = 100

	// a threshold of block height beyond which collections should be re-requested (regardless of the number of blocks for which collection are missing)
	// this is to ensure that if a collection is missing for a long time (in terms of block height) it is eventually re-requested
	missingCollsForAgeThreshold = 100

	// default queue capacity
	defaultQueueCapacity = 10_000

	// processFinalizedBlocksWorkersCount defines the number of workers that
	// concurrently process finalized blocks in the job queue.
	processFinalizedBlocksWorkersCount = 1

	// ensure blocks are processed sequentially by jobqueue
	searchAhead = 1
)

// Engine represents the ingestion engine, used to funnel data from other nodes
// to a centralized location that can be queried by a user
//
// No errors are expected during normal operation.
type Engine struct {
	*component.ComponentManager
	messageHandler            *engine.MessageHandler
	executionReceiptsNotifier engine.Notifier
	executionReceiptsQueue    engine.MessageStore
	// Job queue
	finalizedBlockConsumer *jobqueue.ComponentConsumer
	// Notifier for queue consumer
	finalizedBlockNotifier engine.Notifier

	// txResultErrorMessagesChan is used to fetch and store transaction result error messages for blocks
	txResultErrorMessagesChan chan flow.Identifier

	log   zerolog.Logger // used to log relevant actions with context
	state protocol.State // used to access the  protocol state
	me    module.Local   // used to access local node information

	// storage
	// FIX: remove direct DB access by substituting indexer module
	db                storage.DB
	lockManager       storage.LockManager
	blocks            storage.Blocks
	executionReceipts storage.ExecutionReceipts
	maxReceiptHeight  uint64
	executionResults  storage.ExecutionResults

	collectionSyncer *CollectionSyncer
	// TODO: There's still a need for this metric to be in the ingestion engine rather than collection syncer.
	// Maybe it is a good idea to split it up?
	collectionExecutedMetric module.CollectionExecutedMetric

	txErrorMessagesCore *tx_error_messages.TxErrorMessagesCore
}

var _ network.MessageProcessor = (*Engine)(nil)

// New creates a new access ingestion engine
//
// No errors are expected during normal operation.
func New(
	log zerolog.Logger,
	net network.EngineRegistry,
	state protocol.State,
	me module.Local,
	lockManager storage.LockManager,
	db storage.DB,
	blocks storage.Blocks,
	executionResults storage.ExecutionResults,
	executionReceipts storage.ExecutionReceipts,
	finalizedProcessedHeight storage.ConsumerProgressInitializer,
	collectionSyncer *CollectionSyncer,
	collectionExecutedMetric module.CollectionExecutedMetric,
	txErrorMessagesCore *tx_error_messages.TxErrorMessagesCore,
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

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:                      log.With().Str("engine", "ingestion").Logger(),
		state:                    state,
		me:                       me,
		lockManager:              lockManager,
		db:                       db,
		blocks:                   blocks,
		executionResults:         executionResults,
		executionReceipts:        executionReceipts,
		maxReceiptHeight:         0,
		collectionExecutedMetric: collectionExecutedMetric,
		finalizedBlockNotifier:   engine.NewNotifier(),

		// queue / notifier for execution receipts
		executionReceiptsNotifier: engine.NewNotifier(),
		txResultErrorMessagesChan: make(chan flow.Identifier, 1),
		executionReceiptsQueue:    executionReceiptsQueue,
		messageHandler:            messageHandler,
		txErrorMessagesCore:       txErrorMessagesCore,
		collectionSyncer:          collectionSyncer,
	}

	// jobqueue Jobs object that tracks finalized blocks by height. This is used by the finalizedBlockConsumer
	// to get a sequential list of finalized blocks.
	finalizedBlockReader := jobqueue.NewFinalizedBlockReader(state, blocks)

	defaultIndex, err := e.defaultProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("could not read default finalized processed index: %w", err)
	}

	// create a jobqueue that will process new available finalized block. The `finalizedBlockNotifier` is used to
	// signal new work, which is being triggered on the `processFinalizedBlockJob` handler.
	e.finalizedBlockConsumer, err = jobqueue.NewComponentConsumer(
		e.log.With().Str("module", "ingestion_block_consumer").Logger(),
		e.finalizedBlockNotifier.Channel(),
		finalizedProcessedHeight,
		finalizedBlockReader,
		defaultIndex,
		e.processFinalizedBlockJob,
		processFinalizedBlocksWorkersCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating finalizedBlock jobqueue: %w", err)
	}

	// Add workers
	builder := component.NewComponentManagerBuilder().
		AddWorker(e.collectionSyncer.RequestCollections).
		AddWorker(e.processExecutionReceipts).
		AddWorker(e.runFinalizedBlockConsumer)

	//TODO: should I add a check for nil ptr for collection syncer ? (as done below)

	// If txErrorMessagesCore is provided, add a worker responsible for processing
	// transaction result error messages by receipts. This worker listens for blocks
	// containing execution receipts and processes any associated transaction result
	// error messages. The worker is added only when error message processing is enabled.
	if txErrorMessagesCore != nil {
		builder.AddWorker(e.processTransactionResultErrorMessagesByReceipts)
	}

	e.ComponentManager = builder.Build()

	// register engine with the execution receipt provider
	_, err = net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	return e, nil
}

// defaultProcessedIndex returns the last finalized block height from the protocol state.
//
// The finalizedBlockConsumer utilizes this return height to fetch and consume block jobs from
// jobs queue the first time it initializes.
//
// No errors are expected during normal operation.
func (e *Engine) defaultProcessedIndex() (uint64, error) {
	final, err := e.state.Final().Head()
	if err != nil {
		return 0, fmt.Errorf("could not get finalized height: %w", err)
	}
	return final.Height, nil
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

// processExecutionReceipts is responsible for processing the execution receipts.
// It listens for incoming execution receipts and processes them asynchronously.
func (e *Engine) processExecutionReceipts(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	notifier := e.executionReceiptsNotifier.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
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

		if err := e.handleExecutionReceipt(msg.OriginID, receipt); err != nil {
			return err
		}

		// Notify to fetch and store transaction result error messages for the block.
		// If txErrorMessagesCore is enabled, the receipt's BlockID is sent to trigger
		// transaction error message processing. This step is skipped if error message
		// storage is not enabled.
		if e.txErrorMessagesCore != nil {
			e.txResultErrorMessagesChan <- receipt.BlockID
		}
	}
}

// processTransactionResultErrorMessagesByReceipts handles error messages related to transaction
// results by reading from the error messages channel and processing them accordingly.
//
// This function listens for messages on the txResultErrorMessagesChan channel and
// processes each transaction result error message as it arrives.
//
// No errors are expected during normal operation.
func (e *Engine) processTransactionResultErrorMessagesByReceipts(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case blockID := <-e.txResultErrorMessagesChan:
			err := e.txErrorMessagesCore.FetchErrorMessages(ctx, blockID)
			if err != nil {
				// TODO: we should revisit error handling here.
				// Errors that come from querying the EN and possibly ExecutionNodesForBlockID should be logged and
				// retried later, while others should cause an exception.
				e.log.Error().
					Err(err).
					Msg("error encountered while processing transaction result error messages by receipts")
			}
		}
	}
}

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
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
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(_ channels.Channel, originID flow.Identifier, event interface{}) error {
	return e.process(originID, event)
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated.
// Receives block finalized events from the finalization distributor and forwards them to the finalizedBlockConsumer.
func (e *Engine) OnFinalizedBlock(*model.Block) {
	e.finalizedBlockNotifier.Notify()
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

	// TODO (leothis): to include in the batch update
	// index the block storage with each of the collection guarantee
	err := e.blocks.IndexBlockContainingCollectionGuarantees(block.ID(), flow.GetIDs(block.Payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not index block for collections: %w", err)
	}

	// TODO (leothis): to use a different lock ID
	err = storage.WithLock(e.lockManager, storage.LockInsertOwnReceipt, func(lctx lockctx.Context) error {
		return e.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			// loop through seals and index ID -> result ID
			for _, seal := range block.Payload.Seals {
				err := e.executionResults.BatchIndex(lctx, rw, seal.BlockID, seal.ResultID)
				if err != nil {
					return fmt.Errorf("could not index block for execution result: %w", err)
				}
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("could not index execution results: %w", err)
	}

	e.collectionSyncer.RequestCollectionsForBlock(block.Height, block.Payload.Guarantees)
	e.collectionExecutedMetric.BlockFinalized(block)

	return nil
}

// handleExecutionReceipt persists the execution receipt locally.
// Storing the execution receipt and updates the collection executed metric.
//
// No errors are expected during normal operation.
func (e *Engine) handleExecutionReceipt(_ flow.Identifier, r *flow.ExecutionReceipt) error {
	// persist the execution receipt locally, storing will also index the receipt
	err := e.executionReceipts.Store(r)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	e.collectionExecutedMetric.ExecutionReceiptReceived(r)
	return nil
}
