package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
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
	missingCollsForBlkThreshold = 100

	// a threshold of block height beyond which collections should be re-requested (regardless of the number of blocks for which collection are missing)
	// this is to ensure that if a collection is missing for a long time (in terms of block height) it is eventually re-requested
	missingCollsForAgeThreshold = 100

	// default queue capacity
	defaultQueueCapacity = 10_000

	// how many workers will concurrently process the tasks in the jobqueue
	workersCount = 1

	// ensure blocks are processed sequentially by jobqueue
	searchAhead = 1
)

var (
	defaultCollectionCatchupTimeout               = collectionCatchupTimeout
	defaultCollectionCatchupDBPollInterval        = collectionCatchupDBPollInterval
	defaultFullBlockRefreshInterval               = fullBlockRefreshInterval
	defaultMissingCollsRequestInterval            = missingCollsRequestInterval
	defaultMissingCollsForBlkThreshold            = missingCollsForBlkThreshold
	defaultMissingCollsForAgeThreshold     uint64 = missingCollsForAgeThreshold
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

	// blockIDChan to fetch and store transaction result error messages
	blockIDChan chan flow.Identifier

	log     zerolog.Logger   // used to log relevant actions with context
	state   protocol.State   // used to access the  protocol state
	me      module.Local     // used to access local node information
	request module.Requester // used to request collections

	// storage
	// FIX: remove direct DB access by substituting indexer module
	blocks                         storage.Blocks
	headers                        storage.Headers
	collections                    storage.Collections
	transactions                   storage.Transactions
	executionReceipts              storage.ExecutionReceipts
	maxReceiptHeight               uint64
	executionResults               storage.ExecutionResults
	transactionResultErrorMessages storage.TransactionResultErrorMessages

	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
	// metrics
	collectionExecutedMetric module.CollectionExecutedMetric

	preferredENIdentifiers flow.IdentifierList
	fixedENIdentifiers     flow.IdentifierList

	backend *backend.Backend
	db      *badger.DB
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
	request module.Requester,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionResults storage.ExecutionResults,
	executionReceipts storage.ExecutionReceipts,
	transactionResultErrorMessages storage.TransactionResultErrorMessages,
	collectionExecutedMetric module.CollectionExecutedMetric,
	processedHeight storage.ConsumerProgress,
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter,
	backend *backend.Backend,
	db *badger.DB,
	preferredExecutionNodeIDs []string,
	fixedExecutionNodeIDs []string,
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

	collectionExecutedMetric.UpdateLastFullBlockHeight(lastFullBlockHeight.Value())

	preferredENIdentifiers, err := commonrpc.IdentifierList(preferredExecutionNodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for preferred EN map: %w", err)
	}

	fixedENIdentifiers, err := commonrpc.IdentifierList(fixedExecutionNodeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node id string to Flow Identifier for fixed EN map: %w", err)
	}

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:                            log.With().Str("engine", "ingestion").Logger(),
		state:                          state,
		me:                             me,
		request:                        request,
		blocks:                         blocks,
		headers:                        headers,
		collections:                    collections,
		transactions:                   transactions,
		executionResults:               executionResults,
		executionReceipts:              executionReceipts,
		transactionResultErrorMessages: transactionResultErrorMessages,
		maxReceiptHeight:               0,
		collectionExecutedMetric:       collectionExecutedMetric,
		finalizedBlockNotifier:         engine.NewNotifier(),
		lastFullBlockHeight:            lastFullBlockHeight,

		// queue / notifier for execution receipts
		executionReceiptsNotifier: engine.NewNotifier(),
		blockIDChan:               make(chan flow.Identifier, 1),
		executionReceiptsQueue:    executionReceiptsQueue,
		messageHandler:            messageHandler,
		backend:                   backend,
		db:                        db,
		preferredENIdentifiers:    preferredENIdentifiers,
		fixedENIdentifiers:        fixedENIdentifiers,
	}

	// jobqueue Jobs object that tracks finalized blocks by height. This is used by the finalizedBlockConsumer
	// to get a sequential list of finalized blocks.
	finalizedBlockReader := jobqueue.NewFinalizedBlockReader(state, blocks)

	defaultIndex, err := e.defaultProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("could not read default processed index: %w", err)
	}

	// create a jobqueue that will process new available finalized block. The `finalizedBlockNotifier` is used to
	// signal new work, which is being triggered on the `processFinalizedBlockJob` handler.
	e.finalizedBlockConsumer, err = jobqueue.NewComponentConsumer(
		e.log.With().Str("module", "ingestion_block_consumer").Logger(),
		e.finalizedBlockNotifier.Channel(),
		processedHeight,
		finalizedBlockReader,
		defaultIndex,
		e.processFinalizedBlockJob,
		workersCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating finalizedBlock jobqueue: %w", err)
	}

	// Add workers
	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.processBackground).
		AddWorker(e.processExecutionReceipts).
		AddWorker(e.runFinalizedBlockConsumer).
		AddWorker(e.processTransactionResultErrorMessages).
		Build()

	// register engine with the execution receipt provider
	_, err = net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	return e, nil
}

// defaultProcessedIndex returns the last finalized block height from the protocol state.
//
// The BlockConsumer utilizes this return height to fetch and consume block jobs from
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

// processBackground is a background routine responsible for executing periodic tasks related to block processing and collection retrieval.
// It performs tasks such as updating indexes of processed blocks and requesting missing collections from the network.
// This function runs indefinitely until the context is canceled.
// Periodically, it checks for updates in the last fully processed block index and requests missing collections if necessary.
// Additionally, it checks for missing collections across a range of blocks and requests them if certain thresholds are met.
func (e *Engine) processBackground(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	// context with timeout
	requestCtx, cancel := context.WithTimeout(ctx, defaultCollectionCatchupTimeout)
	defer cancel()

	// request missing collections
	err := e.requestMissingCollections(requestCtx)
	if err != nil {
		e.log.Error().Err(err).Msg("requesting missing collections failed")
	}
	ready()

	updateTicker := time.NewTicker(defaultFullBlockRefreshInterval)
	defer updateTicker.Stop()

	requestTicker := time.NewTicker(defaultMissingCollsRequestInterval)
	defer requestTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		// refresh the LastFullBlockReceived index
		case <-updateTicker.C:
			err := e.updateLastFullBlockReceivedIndex()
			if err != nil {
				ctx.Throw(err)
			}

		// request missing collections from the network
		case <-requestTicker.C:
			err := e.checkMissingCollections()
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
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

		// notify, to fetch and store transaction result error messages
		e.blockIDChan <- receipt.BlockID
	}
}

func (e *Engine) processTransactionResultErrorMessages(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case blockID := <-e.blockIDChan:
			err := e.handleTransactionResultErrorMessages(ctx, blockID)
			if err != nil {
				// if an error reaches this point, it is unexpected
				ctx.Throw(err)
				return
			}
		}
	}
}

func (e *Engine) handleTransactionResultErrorMessages(ctx context.Context, blockID flow.Identifier) error {
	execNodes, err := commonrpc.ExecutionNodesForBlockID(ctx, blockID, e.executionReceipts, e.state, e.log, e.preferredENIdentifiers, e.preferredENIdentifiers)
	if err != nil {
		return fmt.Errorf("failed to select execution nodes: %w", err)
	}

	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}

	resp, execNode, err := e.backend.GetTransactionErrorMessagesFromAnyEN(ctx, execNodes, req)
	if err != nil {
		// continue, we will add functionality to backfill these later
		return nil
	}

	err = e.storeTransactionResultErrorMessages(blockID, resp, execNode)
	if err != nil {
		return fmt.Errorf("could not store error messages: %w", err)
	}

	return nil
}

func (e *Engine) storeTransactionResultErrorMessages(
	blockID flow.Identifier,
	errorMessagesResponses []*execproto.GetTransactionErrorMessagesResponse_Result,
	execNode *flow.IdentitySkeleton,
) error {
	// Write Batch is BadgerDB feature designed for handling lots of writes
	// in efficient and atomic manner, hence pushing all the updates we can
	// as tightly as possible to let Badger manage it.
	// Note, that it does not guarantee atomicity as transactions has size limit,
	// but it's the closest thing to atomicity we could have
	batch := badgerstorage.NewBatch(e.db)

	errorMessages := make([]flow.TransactionResultErrorMessage, len(errorMessagesResponses))

	for _, value := range errorMessagesResponses {
		errorMessage := flow.TransactionResultErrorMessage{
			ErrorMessage:  value.ErrorMessage,
			TransactionID: convert.MessageToIdentifier(value.TransactionId),
			ExecutorID:    execNode.NodeID,
		}
		errorMessages = append(errorMessages, errorMessage)
	}

	err := e.transactionResultErrorMessages.BatchStore(blockID, errorMessages, batch)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	return nil
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

	// skip requesting collections, if this block is below the last full block height
	// this means that either we have already received these collections, or the block
	// may contain unverifiable guarantees (in case this node has just joined the network)
	lastFullBlockHeight := e.lastFullBlockHeight.Value()
	if block.Header.Height <= lastFullBlockHeight {
		e.log.Info().Msgf("skipping requesting collections for finalized block below last full block height (%d<=%d)", block.Header.Height, lastFullBlockHeight)
		return nil
	}

	// queue requesting each of the collections from the collection node
	e.requestCollectionsInFinalizedBlock(block.Payload.Guarantees)

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

// OnCollection handles the response of the collection request made earlier when a block was received.
// No errors expected during normal operations.
func (e *Engine) OnCollection(originID flow.Identifier, entity flow.Entity) {
	collection, ok := entity.(*flow.Collection)
	if !ok {
		e.log.Error().Msgf("invalid entity type (%T)", entity)
		return
	}

	err := indexer.HandleCollection(collection, e.collections, e.transactions, e.log, e.collectionExecutedMetric)
	if err != nil {
		e.log.Error().Err(err).Msg("could not handle collection")
		return
	}
}

// requestMissingCollections requests missing collections for all blocks in the local db storage once at startup
func (e *Engine) requestMissingCollections(ctx context.Context) error {
	var startHeight, endHeight uint64

	// get the height of the last block for which all collections were received
	lastFullHeight := e.lastFullBlockHeight.Value()
	// start from the next block
	startHeight = lastFullHeight + 1

	// end at the finalized block
	finalBlk, err := e.state.Final().Head()
	if err != nil {
		return err
	}
	endHeight = finalBlk.Height

	e.log.Info().
		Uint64("start_height", startHeight).
		Uint64("end_height", endHeight).
		Msg("starting collection catchup")

	// collect all missing collection ids in a map
	var missingCollMap = make(map[flow.Identifier]struct{})

	// iterate through the complete chain and request the missing collections
	for i := startHeight; i <= endHeight; i++ {

		// if deadline exceeded or someone cancelled the context
		if ctx.Err() != nil {
			return fmt.Errorf("failed to complete requests for missing collections: %w", ctx.Err())
		}

		missingColls, err := e.missingCollectionsAtHeight(i)
		if err != nil {
			return fmt.Errorf("failed to retrieve missing collections by height %d during collection catchup: %w", i, err)
		}

		// request the missing collections
		e.requestCollectionsInFinalizedBlock(missingColls)

		// add them to the missing collection id map to track later
		for _, cg := range missingColls {
			missingCollMap[cg.CollectionID] = struct{}{}
		}
	}

	// if no collections were found to be missing we are done.
	if len(missingCollMap) == 0 {
		// nothing more to do
		e.log.Info().Msg("no missing collections found")
		return nil
	}

	// the collection catchup needs to happen ASAP when the node starts up. Hence, force the requester to dispatch all request
	e.request.Force()

	// track progress of retrieving all the missing collections by polling the db periodically
	ticker := time.NewTicker(defaultCollectionCatchupDBPollInterval)
	defer ticker.Stop()

	// while there are still missing collections, keep polling
	for len(missingCollMap) > 0 {
		select {
		case <-ctx.Done():
			// context may have expired
			return fmt.Errorf("failed to complete collection retreival: %w", ctx.Err())
		case <-ticker.C:

			// log progress
			e.log.Info().
				Int("total_missing_collections", len(missingCollMap)).
				Msg("retrieving missing collections...")

			var foundColls []flow.Identifier
			// query db to find if collections are still missing
			for collID := range missingCollMap {
				found, err := e.haveCollection(collID)
				if err != nil {
					return err
				}
				// if collection found in local db, remove it from missingColls later
				if found {
					foundColls = append(foundColls, collID)
				}
			}

			// update the missingColls list by removing collections that have now been received
			for _, c := range foundColls {
				delete(missingCollMap, c)
			}
		}
	}

	e.log.Info().Msg("collection catchup done")
	return nil
}

// updateLastFullBlockReceivedIndex finds the next highest height where all previous collections
// have been indexed, and updates the LastFullBlockReceived index to that height
func (e *Engine) updateLastFullBlockReceivedIndex() error {
	lastFullHeight := e.lastFullBlockHeight.Value()

	finalBlk, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}
	finalizedHeight := finalBlk.Height

	// track the latest contiguous full height
	newLastFullHeight, err := e.lowestHeightWithMissingCollection(lastFullHeight, finalizedHeight)
	if err != nil {
		return fmt.Errorf("failed to find last full block received height: %w", err)
	}

	// if more contiguous blocks are now complete, update db
	if newLastFullHeight > lastFullHeight {
		err := e.lastFullBlockHeight.Set(newLastFullHeight)
		if err != nil {
			return fmt.Errorf("failed to update last full block height: %w", err)
		}

		e.collectionExecutedMetric.UpdateLastFullBlockHeight(newLastFullHeight)

		e.log.Debug().
			Uint64("last_full_block_height", newLastFullHeight).
			Msg("updated LastFullBlockReceived index")
	}

	return nil
}

// lowestHeightWithMissingCollection returns the lowest height that is missing collections
func (e *Engine) lowestHeightWithMissingCollection(lastFullHeight, finalizedHeight uint64) (uint64, error) {
	newLastFullHeight := lastFullHeight

	for i := lastFullHeight + 1; i <= finalizedHeight; i++ {
		missingColls, err := e.missingCollectionsAtHeight(i)
		if err != nil {
			return 0, err
		}

		// return when we find the first block with missing collections
		if len(missingColls) > 0 {
			return newLastFullHeight, nil
		}

		newLastFullHeight = i
	}

	return newLastFullHeight, nil
}

// checkMissingCollections requests missing collections if the number of blocks missing collections
// have reached the defaultMissingCollsForBlkThreshold value.
func (e *Engine) checkMissingCollections() error {
	lastFullHeight := e.lastFullBlockHeight.Value()

	finalBlk, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to get finalized block: %w", err)
	}
	finalizedHeight := finalBlk.Height

	// number of blocks with missing collections
	incompleteBlksCnt := 0

	// collect all missing collections
	var allMissingColls []*flow.CollectionGuarantee

	// start from the next block till we either hit the finalized block or cross the max collection missing threshold
	for i := lastFullHeight + 1; i <= finalizedHeight && incompleteBlksCnt < defaultMissingCollsForBlkThreshold; i++ {
		missingColls, err := e.missingCollectionsAtHeight(i)
		if err != nil {
			return fmt.Errorf("failed to find missing collections at height %d: %w", i, err)
		}

		if len(missingColls) == 0 {
			continue
		}

		incompleteBlksCnt++

		allMissingColls = append(allMissingColls, missingColls...)
	}

	// additionally, if more than threshold blocks have missing collections OR collections are
	// missing since defaultMissingCollsForAgeThreshold, re-request those collections
	if incompleteBlksCnt >= defaultMissingCollsForBlkThreshold ||
		(finalizedHeight-lastFullHeight) > defaultMissingCollsForAgeThreshold {
		// warn log since this should generally not happen
		e.log.Warn().
			Uint64("finalized_height", finalizedHeight).
			Uint64("last_full_blk_height", lastFullHeight).
			Int("missing_collection_blk_count", incompleteBlksCnt).
			Int("missing_collection_count", len(allMissingColls)).
			Msg("re-requesting missing collections")
		e.requestCollectionsInFinalizedBlock(allMissingColls)
	}

	return nil
}

// missingCollectionsAtHeight returns all missing collection guarantees at a given height
func (e *Engine) missingCollectionsAtHeight(h uint64) ([]*flow.CollectionGuarantee, error) {
	block, err := e.blocks.ByHeight(h)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve block by height %d: %w", h, err)
	}

	var missingColls []*flow.CollectionGuarantee
	for _, guarantee := range block.Payload.Guarantees {
		collID := guarantee.CollectionID
		found, err := e.haveCollection(collID)
		if err != nil {
			return nil, err
		}
		if !found {
			missingColls = append(missingColls, guarantee)
		}
	}
	return missingColls, nil
}

// haveCollection looks up the collection from the collection db with collID
func (e *Engine) haveCollection(collID flow.Identifier) (bool, error) {
	_, err := e.collections.LightByID(collID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("failed to retrieve collection %s: %w", collID.String(), err)
}

// requestCollectionsInFinalizedBlock registers collection requests with the requester engine
func (e *Engine) requestCollectionsInFinalizedBlock(missingColls []*flow.CollectionGuarantee) {
	for _, cg := range missingColls {
		guarantors, err := protocol.FindGuarantors(e.state, cg)
		if err != nil {
			// failed to find guarantors for guarantees contained in a finalized block is fatal error
			e.log.Fatal().Err(err).Msgf("could not find guarantors for guarantee %v", cg.ID())
		}
		e.request.EntityByID(cg.ID(), filter.HasNodeID[flow.Identity](guarantors...))
	}
}
