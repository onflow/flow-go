// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// time to wait for the all the missing collections to be received at node startup
const collectionCatchupTimeout = 30 * time.Second

// time to poll the storage to check if missing collections have been received
const collectionCatchupDBPollInterval = 10 * time.Millisecond

// time to update the FullBlockHeight index
const fullBlockRefreshInterval = 1 * time.Second

// time to request missing collections from the network
const missingCollsRequestInterval = 1 * time.Minute

// a threshold of number of blocks with missing collections beyond which collections should be re-requested
// this is to prevent spamming the collection nodes with request
const missingCollsForBlkThreshold = 100

// a threshold of block height beyond which collections should be re-requested (regardless of the number of blocks for which collection are missing)
// this is to ensure that if a collection is missing for a long time (in terms of block height) it is eventually re-requested
const missingCollsForAgeThreshold = 100

// default queue capacity
const defaultQueueCapacity = 10_000

var defaultCollectionCatchupTimeout = collectionCatchupTimeout
var defaultCollectionCatchupDBPollInterval = collectionCatchupDBPollInterval
var defaultFullBlockRefreshInterval = fullBlockRefreshInterval
var defaultMissingCollsRequestInterval = missingCollsRequestInterval
var defaultMissingCollsForBlkThreshold = missingCollsForBlkThreshold
var defaultMissingCollsForAgeThreshold uint64 = missingCollsForAgeThreshold

// Engine represents the ingestion engine, used to funnel data from other nodes
// to a centralized location that can be queried by a user
type Engine struct {
	*component.ComponentManager
	messageHandler            *engine.MessageHandler
	executionReceiptsNotifier engine.Notifier
	executionReceiptsQueue    engine.MessageStore
	finalizedBlockNotifier    engine.Notifier
	finalizedBlockQueue       engine.MessageStore

	log     zerolog.Logger   // used to log relevant actions with context
	state   protocol.State   // used to access the  protocol state
	me      module.Local     // used to access local node information
	request module.Requester // used to request collections

	// storage
	// FIX: remove direct DB access by substituting indexer module
	blocks            storage.Blocks
	headers           storage.Headers
	collections       storage.Collections
	transactions      storage.Transactions
	executionReceipts storage.ExecutionReceipts
	maxReceiptHeight  uint64
	executionResults  storage.ExecutionResults

	// metrics
	metrics                    module.AccessMetrics
	collectionsToMarkFinalized *stdmap.Times
	collectionsToMarkExecuted  *stdmap.Times
	blocksToMarkExecuted       *stdmap.Times
}

// New creates a new access ingestion engine
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
	accessMetrics module.AccessMetrics,
	collectionsToMarkFinalized *stdmap.Times,
	collectionsToMarkExecuted *stdmap.Times,
	blocksToMarkExecuted *stdmap.Times,
) (*Engine, error) {
	executionReceiptsRawQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create execution receipts queue: %w", err)
	}

	executionReceiptsQueue := &engine.FifoMessageStore{FifoQueue: executionReceiptsRawQueue}

	finalizedBlocksRawQueue, err := fifoqueue.NewFifoQueue(defaultQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("could not create finalized block queue: %w", err)
	}

	finalizedBlocksQueue := &engine.FifoMessageStore{FifoQueue: finalizedBlocksRawQueue}

	messageHandler := engine.NewMessageHandler(
		log,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*model.Block)
				return ok
			},
			Store: finalizedBlocksQueue,
		},
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
		log:                        log.With().Str("engine", "ingestion").Logger(),
		state:                      state,
		me:                         me,
		request:                    request,
		blocks:                     blocks,
		headers:                    headers,
		collections:                collections,
		transactions:               transactions,
		executionResults:           executionResults,
		executionReceipts:          executionReceipts,
		maxReceiptHeight:           0,
		metrics:                    accessMetrics,
		collectionsToMarkFinalized: collectionsToMarkFinalized,
		collectionsToMarkExecuted:  collectionsToMarkExecuted,
		blocksToMarkExecuted:       blocksToMarkExecuted,

		// queue / notifier for execution receipts
		executionReceiptsNotifier: engine.NewNotifier(),
		executionReceiptsQueue:    executionReceiptsQueue,

		// queue / notifier for finalized blocks
		finalizedBlockNotifier: engine.NewNotifier(),
		finalizedBlockQueue:    finalizedBlocksQueue,

		messageHandler: messageHandler,
	}

	// Add workers
	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.processBackground).
		AddWorker(e.processExecutionReceipts).
		AddWorker(e.processFinalizedBlocks).
		Build()

	// register engine with the execution receipt provider
	_, err = net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	return e, nil
}

func (e *Engine) Start(parent irrecoverable.SignalerContext) {
	err := e.initLastFullBlockHeightIndex()
	if err != nil {
		parent.Throw(fmt.Errorf("unexpected error initializing full block index: %w", err))
	}

	e.ComponentManager.Start(parent)
}

// initializeLastFullBlockHeightIndex initializes the index of full blocks
// (blocks for which we have ingested all collections) to the root block height.
// This means that the Access Node will ingest all collections for all blocks
// ingested after state bootstrapping is complete (all blocks received from the network).
// If the index has already been initialized, this is a no-op.
// No errors are expected during normal operation.
func (e *Engine) initLastFullBlockHeightIndex() error {
	rootBlock, err := e.state.Params().FinalizedRoot()
	if err != nil {
		return fmt.Errorf("failed to get root block: %w", err)
	}

	// insert is a noop if the index has already been initialized and no error is returned
	err = e.blocks.InsertLastFullBlockHeightIfNotExists(rootBlock.Height)
	if err != nil {
		return fmt.Errorf("failed to update last full block height during ingestion engine startup: %w", err)
	}

	lastFullHeight, err := e.blocks.GetLastFullBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to get last full block height during ingestion engine startup: %w", err)
	}

	e.metrics.UpdateLastFullBlockHeight(lastFullHeight)

	return nil
}

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
	}

}

func (e *Engine) processFinalizedBlocks(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	notifier := e.finalizedBlockNotifier.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-notifier:
			_ = e.processAvailableFinalizedBlocks(ctx)
		}
	}
}

func (e *Engine) processAvailableFinalizedBlocks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := e.finalizedBlockQueue.Get()
		if !ok {
			return nil
		}

		hb := msg.Payload.(*model.Block)
		blockID := hb.BlockID

		if err := e.processFinalizedBlock(blockID); err != nil {
			e.log.Error().Err(err).Hex("block_id", blockID[:]).Msg("failed to process block")
			continue
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
	case *model.Block:
		err := e.messageHandler.Process(originID, event)
		e.finalizedBlockNotifier.Notify()
		return err
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	err := e.process(e.me.NodeID(), event)
	if err != nil {
		engine.LogError(e.log, err)
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(_ channels.Channel, originID flow.Identifier, event interface{}) {
	err := e.process(originID, event)
	if err != nil {
		engine.LogError(e.log, err)
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(_ channels.Channel, originID flow.Identifier, event interface{}) error {
	return e.process(originID, event)
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated
func (e *Engine) OnFinalizedBlock(hb *model.Block) {
	_ = e.ProcessLocal(hb)
}

// processBlock handles an incoming finalized block.
func (e *Engine) processFinalizedBlock(blockID flow.Identifier) error {

	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	block, err := e.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("failed to lookup block: %w", err)
	}

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// TODO: substitute an indexer module as layer between engine and storage

	// index the block storage with each of the collection guarantee
	err = e.blocks.IndexBlockForCollections(block.Header.ID(), flow.GetIDs(block.Payload.Guarantees))
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
	lastFullBlockHeight, err := e.blocks.GetLastFullBlockHeight()
	if err != nil {
		return fmt.Errorf("could not get last full block height: %w", err)
	}

	if block.Header.Height <= lastFullBlockHeight {
		e.log.Info().Msgf("skipping requesting collections for finalized block below last full block height (%d<=%d)", block.Header.Height, lastFullBlockHeight)
		return nil
	}

	// queue requesting each of the collections from the collection node
	e.requestCollectionsInFinalizedBlock(block.Payload.Guarantees)

	e.trackFinalizedMetricForBlock(block)

	return nil
}

func (e *Engine) trackFinalizedMetricForBlock(block *flow.Block) {
	// TODO: lookup actual finalization time by looking at the block finalizing `b`
	now := time.Now().UTC()
	blockID := block.ID()

	// mark all transactions as finalized
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := e.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			e.collectionsToMarkFinalized.Add(g.CollectionID, now)
			continue
		} else if err != nil {
			e.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx finalized metric: finalized collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			e.metrics.TransactionFinalized(t, now)
		}
	}

	if ti, found := e.blocksToMarkExecuted.ByID(blockID); found {
		e.trackExecutedMetricForBlock(block, ti)
		e.metrics.UpdateExecutionReceiptMaxHeight(block.Header.Height)
		e.blocksToMarkExecuted.Remove(blockID)
	}
}

func (e *Engine) handleExecutionReceipt(_ flow.Identifier, r *flow.ExecutionReceipt) error {
	// persist the execution receipt locally, storing will also index the receipt
	err := e.executionReceipts.Store(r)
	if err != nil {
		return fmt.Errorf("failed to store execution receipt: %w", err)
	}

	e.trackExecutionReceiptMetrics(r)
	return nil
}

func (e *Engine) trackExecutionReceiptMetrics(r *flow.ExecutionReceipt) {
	// TODO add actual execution time to execution receipt?
	now := time.Now().UTC()

	// retrieve the block
	// TODO: consider using storage.Index.ByBlockID, the index contains collection id and seals ID
	b, err := e.blocks.ByID(r.ExecutionResult.BlockID)

	if errors.Is(err, storage.ErrNotFound) {
		e.blocksToMarkExecuted.Add(r.ExecutionResult.BlockID, now)
		return
	}

	if err != nil {
		e.log.Warn().Err(err).Msg("could not track tx executed metric: executed block not found locally")
		return
	}

	e.metrics.UpdateExecutionReceiptMaxHeight(b.Header.Height)

	e.trackExecutedMetricForBlock(b, now)
}

func (e *Engine) trackExecutedMetricForBlock(block *flow.Block, ti time.Time) {
	// mark all transactions as executed
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := e.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			e.collectionsToMarkExecuted.Add(g.CollectionID, ti)
			continue
		} else if err != nil {
			e.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx executed metric: executed collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			e.metrics.TransactionExecuted(t, ti)
		}
	}
}

func (e *Engine) trackExecutedMetricForCollection(light *flow.LightCollection) {
	if ti, found := e.collectionsToMarkFinalized.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			e.metrics.TransactionFinalized(t, ti)
		}
		e.collectionsToMarkFinalized.Remove(light.ID())
	}

	if ti, found := e.collectionsToMarkExecuted.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			e.metrics.TransactionExecuted(t, ti)
		}
		e.collectionsToMarkExecuted.Remove(light.ID())
	}
}

// handleCollection handles the response of the a collection request made earlier when a block was received
func (e *Engine) handleCollection(_ flow.Identifier, entity flow.Entity) error {

	// convert the entity to a strictly typed collection
	collection, ok := entity.(*flow.Collection)
	if !ok {
		return fmt.Errorf("invalid entity type (%T)", entity)
	}

	light := collection.Light()

	e.trackExecutedMetricForCollection(&light)

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// store the light collection (collection minus the transaction body - those are stored separately)
	// and add transaction ids as index
	err := e.collections.StoreLightAndIndexByTransaction(&light)
	if err != nil {
		// ignore collection if already seen
		if errors.Is(err, storage.ErrAlreadyExists) {
			e.log.Debug().
				Hex("collection_id", logging.Entity(light)).
				Msg("collection is already seen")
			return nil
		}
		return err
	}

	// now store each of the transaction body
	for _, tx := range collection.Transactions {
		err := e.transactions.Store(tx)
		if err != nil {
			return fmt.Errorf("could not store transaction (%x): %w", tx.ID(), err)
		}
	}

	return nil
}

func (e *Engine) OnCollection(originID flow.Identifier, entity flow.Entity) {
	err := e.handleCollection(originID, entity)
	if err != nil {
		e.log.Error().Err(err).Msg("could not handle collection")
		return
	}
}

// requestMissingCollections requests missing collections for all blocks in the local db storage once at startup
func (e *Engine) requestMissingCollections(ctx context.Context) error {

	var startHeight, endHeight uint64

	// get the height of the last block for which all collections were received
	lastFullHeight, err := e.blocks.GetLastFullBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to complete requests for missing collections: %w", err)
	}

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
	lastFullHeight, err := e.blocks.GetLastFullBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to get last full block height: %w", err)
	}

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
		err = e.blocks.UpdateLastFullBlockHeight(newLastFullHeight)
		if err != nil {
			return fmt.Errorf("failed to update last full block height")
		}

		e.metrics.UpdateLastFullBlockHeight(newLastFullHeight)

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
	lastFullHeight, err := e.blocks.GetLastFullBlockHeight()
	if err != nil {
		return err
	}

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
		e.request.EntityByID(cg.ID(), filter.HasNodeID(guarantors...))
	}
}
