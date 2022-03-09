package requester

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/storage"
)

const (
	// Timeout for fetching ExecutionData from the db/network
	fetchTimeout = 5 * time.Minute

	// Number of goroutines to use for downloading new ExecutionData from the network.
	fetchWorkers = 4

	// The number of ExecutionData objects to keep when waiting to send notifications. Dropped
	// data is refetched from disk.
	executionDataCacheSize = 50

	// Max number of block finalization notifications to enqueue. Dropped notifications are
	// backfilled once a new finalized block notification is processed.
	finalizationQueueLength = 2

	// Note: this must be greater than fetchWorkers, otherwise the retry queue could overflow
	// resulting in lost retry requests
	fetchQueueLength = 500
)

var ErrRequesterHalted = errors.New("requester was halted due to invalid data")

// ExecutionDataRequester downloads ExecutionData for newly sealed blocks from the network using the
// ExecutionDataService. The requester has the following priorities:
//   1. ensure execution state is as widely distributed as possible among the network participants
//   2. use the execution state to build a local index
// The #1 priority of this component is to fetch ExecutionData for as many blocks as possible, making
// them available to other nodes in the network. This ensures execution state is available for all
// nodes, and reduces network load on the execution nodes that source the data. The secondary priority
// is to consume the data locally.
// Focusing on fetching and processing blocks sequentially we simplify the implementation at the cost
// of reducing the number of nodes seeding each block.
type ExecutionDataRequester interface {
	component.Component
	OnBlockFinalized(flow.Identifier)
	AddOnExecutionDataFetchedConsumer(fn ExecutionDataReceivedCallback)
}

type executionDataRequesterImpl struct {
	component.Component
	cm  *component.ComponentManager
	ds  datastore.Batching
	bs  network.BlobService
	eds state_synchronization.ExecutionDataService
	log zerolog.Logger

	// Local db objects
	blocks  storage.Blocks
	results storage.ExecutionResults

	// The first block for which to request ExecutionData
	rootBlock *flow.Block

	// List of callbacks to call when ExecutionData is successfully fetched for a block
	consumers []ExecutionDataReceivedCallback

	// finalizedBlockQueue accepts new finalized blocks to prevent blocking in the OnBlockFinalized
	// callback
	finalizedBlocks chan flow.Identifier

	// fetchQueue accepts new fetch requests, which are consumed by a pool of fetch workers.
	fetchRequests chan fetchRequest

	// fetchRetryQueue accepts fetch retry requests, which are also consumed by the same pool of
	// fetch workers as fetchQueue, however fetchRetryQueue takes priority.
	fetchRetryRequests chan fetchRequest

	// Notifiers for queue consumers
	notifications chan fetchRequest

	cache      *executionDataCache
	status     *status
	consumerMu sync.RWMutex

	startupCheck bool
}

type fetchRequest struct {
	blockID       flow.Identifier
	resultID      flow.Identifier
	height        uint64
	executionData *state_synchronization.ExecutionData
}

type ExecutionDataReceivedCallback func(*state_synchronization.ExecutionData)

// NewexecutionDataRequesterImpl creates a new execution data requester engine
func NewExecutionDataRequester(
	log zerolog.Logger,
	edsMetrics module.ExecutionDataServiceMetrics,
	datastore datastore.Batching,
	blobservice network.BlobService,
	eds state_synchronization.ExecutionDataService,
	rootBlock *flow.Block,
	blocks storage.Blocks,
	results storage.ExecutionResults,
	startupCheck bool,
) (ExecutionDataRequester, error) {
	e := &executionDataRequesterImpl{
		log:          log.With().Str("component", "execution_data_requester").Logger(),
		ds:           datastore,
		bs:           blobservice,
		eds:          eds,
		rootBlock:    rootBlock,
		blocks:       blocks,
		results:      results,
		startupCheck: startupCheck,

		cache:  newExecutionDataCache(executionDataCacheSize),
		status: &status{db: datastore},

		finalizedBlocks:    make(chan flow.Identifier, finalizationQueueLength),
		fetchRequests:      make(chan fetchRequest, fetchQueueLength),
		fetchRetryRequests: make(chan fetchRequest, fetchQueueLength),
		notifications:      make(chan fetchRequest, fetchQueueLength),
	}

	builder := component.NewComponentManagerBuilder().
		AddWorker(e.finalizedBlockProcessor).
		AddWorker(e.notificationProcessor)

	for i := 0; i < fetchWorkers; i++ {
		builder.AddWorker(e.fetchRequestProcessor)
	}

	e.cm = builder.Build()
	e.Component = e.cm

	return e, nil
}

// AddOnExecutionDataFetchedConsumer adds a callback to be called when a new ExecutionData is received
// Callback Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
func (e *executionDataRequesterImpl) AddOnExecutionDataFetchedConsumer(fn ExecutionDataReceivedCallback) {
	e.consumerMu.Lock()
	defer e.consumerMu.Unlock()
	e.consumers = append(e.consumers, fn)
}

func (e *executionDataRequesterImpl) OnBlockFinalized(blockID flow.Identifier) {
	logger := e.log.With().Str("finalized_block_id", blockID.String()).Logger()

	// stop accepting new blocks if the component is shutting down
	if util.CheckClosed(e.cm.ShutdownSignal()) {
		logger.Warn().Msg("ignoring finalized block. component is shutting down")
		return
	}

	// don't accept notifications if the component hasn't finished bootstrapping
	if !util.CheckClosed(e.Ready()) {
		logger.Debug().Msg("ignoring finalized block. component is starting up")
		return
	}

	logger.Debug().Msg("received finalized block notification")

	select {
	case e.finalizedBlocks <- blockID:
	default:
		logger.Warn().Msg("finalized block queue is full")
	}
}

// finalizedBlockProcessor runs the main process that processes finalized block notifications and
// requests ExecutionData for each block seal
func (e *executionDataRequesterImpl) finalizedBlockProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-e.eds.Ready()
	ready()

	// Boostrapping happens after the node is ready since it should not block node startup
	err := e.bootstrap(ctx)

	// Any error (except halts) should crash the node
	if err != nil && errors.Is(err, ErrRequesterHalted) {
		ctx.Throw(err)
	}

	// Start ingesting new finalized block notifications
	e.finalizationProcessingLoop(ctx)
}

func (e *executionDataRequesterImpl) fetchRequestProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-e.eds.Ready()
	ready()

	e.fetchRequestProcessingLoop(ctx)
}

func (e *executionDataRequesterImpl) notificationProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	// the notifier will use the ExecutionDataService to fetch blocks that aren't in the cache, so
	// it must be available
	<-e.eds.Ready()
	ready()

	e.notificationProcessingLoop(ctx)
}

// finalizationProcessingLoop waits for finalized block notifications, then processes all available
// blocks in the queue
func (e *executionDataRequesterImpl) finalizationProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if util.CheckClosed(ctx.Done()) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case blockID := <-e.finalizedBlocks:
			e.processFinalizedBlock(ctx, blockID)
		}
	}
}

func (e *executionDataRequesterImpl) processFinalizedBlock(ctx irrecoverable.SignalerContext, blockID flow.Identifier) {
	block, err := e.blocks.ByID(blockID)

	// block must be in the db, otherwise there's a problem with the state
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to lookup finalized block %s in protocol state db: %w", blockID, err))
	}

	logger := e.log.With().Str("finalized_block_id", block.ID().String()).Logger()

	// loop through all finalized blocks since the last processed block, and extract all seals
	lastHeight := e.status.lastProcessed

	logger.Debug().
		Uint64("start_height", lastHeight+1).
		Uint64("end_height", block.Header.Height).
		Msg("checking for seals")

	for height := lastHeight + 1; height <= block.Header.Height; height++ {
		logger.Debug().Uint64("height", height).Msg("processing height")
		err := e.processSealsFromHeight(ctx, height)

		if err != nil {
			ctx.Throw(fmt.Errorf("failed to process seals from height %d: %w", height, err))
		}

		e.status.lastProcessed = height
	}

	logger.Debug().Msg("done processing")
}

func (e *executionDataRequesterImpl) processSealsFromHeight(ctx irrecoverable.SignalerContext, height uint64) error {
	block, err := e.blocks.ByHeight(height)
	if err != nil {
		return fmt.Errorf("failed to get block: %w", err)
	}

	logger := e.log.With().Str("finalized_block_id", block.ID().String()).Uint64("finalized_block_height", height).Logger()

	if len(block.Payload.Seals) == 0 {
		logger.Debug().Msg("no seals in block")
		return nil
	}

	logger.Debug().Msgf("checking %d seals in block", len(block.Payload.Seals))

	// Find all seals in the block and sort them by height (ascending). This helps with processing
	// lower height first since notifications are sent in order and seals are not guaranteed to
	// be sorted

	requests, err := e.requestsFromSeals(block.Payload.Seals)
	if err != nil {
		return err
	}

	logger.Debug().Msgf("found %d seals in block", len(requests))

	sort.Slice(requests, func(i, j int) bool {
		return requests[i].height < requests[j].height
	})

	// Send all blocks to fetch workers (blocking to apply backpressure)
	for _, request := range requests {
		logger.Debug().Msgf("enqueueing fetch request for block %d", request.height)

		select {
		case <-ctx.Done():
			return nil
		case e.fetchRequests <- request:
		}
	}

	return nil
}

func (e *executionDataRequesterImpl) requestsFromSeals(seals []*flow.Seal) ([]fetchRequest, error) {
	requests := []fetchRequest{}

	for _, seal := range seals {
		sealedBlock, err := e.blocks.ByID(seal.BlockID)

		if err != nil {
			return nil, fmt.Errorf("failed to lookup sealed block in protocol state db: %w", err)
		}

		requests = append(requests, fetchRequest{
			blockID:  seal.BlockID,
			height:   sealedBlock.Header.Height,
			resultID: seal.ResultID,
		})
	}

	return requests, nil
}

// Fetch Worker Methods

func (e *executionDataRequesterImpl) fetchRequestProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if util.CheckClosed(ctx.Done()) {
			return
		}

		select {
		case request := <-e.fetchRetryRequests:
			e.processFetchRequest(ctx, request)
			continue
		default:
		}

		select {
		case <-ctx.Done():
			return
		case request := <-e.fetchRequests:
			e.processFetchRequest(ctx, request)
		}
	}
}

func (e *executionDataRequesterImpl) processFetchRequest(ctx irrecoverable.SignalerContext, request fetchRequest) error {
	logger := e.log.With().Str("block_id", request.blockID.String()).Logger()

	logger.Debug().Msgf("processing fetch request for block %d", request.height)

	// The ExecutionResult contains the root CID for the block's execution data
	result, err := e.results.ByBlockID(request.blockID)

	// By the time the block is sealed, the ExecutionResult must be in the db
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to lookup execution result for block: %w", err))
	}

	executionData, err := e.fetchExecutionData(ctx, result.ExecutionDataID)

	if errors.Is(err, &state_synchronization.MalformedDataError{}) || errors.Is(err, &state_synchronization.BlobSizeLimitExceededError{}) {
		// This means an execution result was sealed with an invalid execution data id (invalid data).
		// Eventually, verification nodes will verify that the execution data id is valid, and not sign the receipt

		// TODO: add metric
		logger.Error().Err(err).
			Str("execution_data_id", result.ExecutionDataID.String()).
			Msg("HALTING REQUESTER: invalid execution data found")

		e.status.Halt(ctx)
		ctx.Throw(ErrRequesterHalted)
	}

	if err != nil {
		logger.Error().Err(err).Msg("failed to get execution data for block")

		select {
		case e.fetchRetryRequests <- request:
		default:
			// Since the retry queue is always checked first, there can be at most fetchWorkers + 1
			// outstanding retry requests. In practice, this can only happen if the retry channel's
			// buffer size is misconfigured.
			logger.Error().Msg("fetch retry queue is full")
			return nil
		}
		return nil
	}
	logger.Debug().Msgf("Fetched execution data for block %d", request.height)

	e.status.Fetched(ctx, request.height, request.blockID)
	request.executionData = executionData

	logger.Debug().Msgf("Enqueueing notification request for execution data for block %d", request.height)
	select {
	case <-ctx.Done():
		return nil
	case e.notifications <- request:
	}
	logger.Debug().Msgf("Sent requested notification for execution data for block %d", request.height)

	return nil
}

// fetchExecutionData fetches the ExecutionData by its ID, using fetchTimeout
func (e *executionDataRequesterImpl) fetchExecutionData(signalerCtx irrecoverable.SignalerContext, executionDataID flow.Identifier) (*state_synchronization.ExecutionData, error) {
	ctx, cancel := context.WithTimeout(signalerCtx, fetchTimeout)
	defer cancel()

	// Fetch the ExecutionData for blockID from the blobstore. If it doesn't exist locally, it will
	// be fetched from the network.
	return e.eds.Get(ctx, executionDataID)
}

// Notification Worker Methods

func (e *executionDataRequesterImpl) notificationProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if util.CheckClosed(ctx.Done()) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case request := <-e.notifications:
			e.processNotification(ctx, request)
		}
	}
}

// TODO: need a loop here to catch up if it ever falls behind
func (e *executionDataRequesterImpl) processNotification(ctx irrecoverable.SignalerContext, request fetchRequest) {
	e.log.Debug().Msgf("processing notification request for block %d", request.height)

	next, _, _ := e.status.NextNotification()

	// if this isn't a duplicate notification, cache it
	if next <= request.height {
		e.log.Debug().Msgf("adding execution data to cache for height %d", request.height)
		accepted := e.cache.Put(request.height, request.executionData)
		if !accepted {
			e.log.Warn().Msg("execution data cache is full")
		}
		e.log.Debug().Msgf("cache %#v", e.cache.heights)
	}

	// process all available notifications
	e.sendAllAvailableNotifications(ctx)
}

func (e *executionDataRequesterImpl) sendAllAvailableNotifications(ctx irrecoverable.SignalerContext) {
	for {
		next, blockID, ok := e.status.NextNotification()

		// we haven't finished fetching the next block to notify
		if !ok {
			e.log.Debug().Msgf("waiting to notify for block %d", next)
			e.log.Debug().Msgf("lastNotified: %d, LastReceived: %d, missing: %#v",
				e.status.lastNotified,
				e.status.LastReceived,
				e.status.MissingHeights(),
			)
			return
		}

		e.log.Debug().Msgf("notifying for block %d", next)

		executionData, ok := e.cache.Get(next)

		if !ok {
			e.log.Debug().Msgf("execution data not in cache for block %d", next)
			e.log.Debug().Msgf("cache %#v", e.cache.heights)
			// get it from disk
			result, err := e.results.ByBlockID(blockID)
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to lookup execution result for block: %w", err))
			}

			executionData, err = e.eds.Get(ctx, result.ExecutionDataID)

			if err != nil {
				// At this point the data has been downloaded and validated, so it should be available
				// TODO: handle this better. should this just delete the data and refetch?
				ctx.Throw(fmt.Errorf("failed to get execution data for block: %w", err))
			}
		}

		// send notifications
		e.notifyConsumers(executionData)

		// update notification manifest that we've sent notifications for this block height, and cleanup the cache
		e.status.Notified(next)
		e.cache.Delete(next)

		e.log.Debug().Msgf("removing ed cache data for height %d", next)
		e.log.Debug().Msgf("cache %#v", e.cache.heights)
	}
}

func (e *executionDataRequesterImpl) notifyConsumers(executionData *state_synchronization.ExecutionData) {
	e.consumerMu.RLock()
	defer e.consumerMu.RUnlock()

	for _, fn := range e.consumers {
		fn(executionData)
	}
}

// Bootstrap Methods

func (e *executionDataRequesterImpl) bootstrap(ctx irrecoverable.SignalerContext) error {
	err := e.loadSyncState(ctx)
	if err != nil {
		return err
	}

	// on run datastore check if enabled
	if !e.startupCheck {
		return nil
	}

	// TODO: if this is a fresh start, we'll need to provide the latest sealed block height as the
	// target to catch up to before starting the finalization loop

	return e.checkDatastore(ctx, e.status.LastReceived)
}

func (e *executionDataRequesterImpl) loadSyncState(ctx irrecoverable.SignalerContext) error {
	err := e.status.Load(ctx)
	if err != nil {
		e.log.Error().Err(err).Msg("failed to load notification state. using default")
	}

	if e.status.Halted {
		e.log.Error().Msg("HALTING REQUESTER: requester was halted on a previous run due to invalid data")
		ctx.Throw(ErrRequesterHalted)
	}

	// defaults to the start block when booting with a fresh db
	if e.status.LastReceived == 0 {
		e.status.LastReceived = e.rootBlock.Header.Height
		e.log.Debug().Msgf("setting LastReceived to root block height: %d", e.status.LastReceived)
	}

	// at boot, we only have the start block and last block received. We can scan through the db
	// to find the other metrics.
	e.status.lastNotified = e.status.LastReceived
	e.status.lastProcessed = e.status.LastReceived

	return nil
}

func (e *executionDataRequesterImpl) checkDatastore(ctx irrecoverable.SignalerContext, lastSealedHeight uint64) error {
	genesis := e.rootBlock.Header.Height

	// Search from genesis to the lastSealedHeight, and confirm data is still available for all heights
	// Update the notification state based on the data in the db
	for height := genesis; height <= lastSealedHeight; height++ {
		if util.CheckClosed(ctx.Done()) {
			return nil
		}

		block, err := e.blocks.ByHeight(height)
		if err != nil {
			// TODO: does it make sense to crash? what happens if the LastReceived value in the db is just wrong?
			return fmt.Errorf("failed to get block for height %d: %w", height, err)
		}

		result, err := e.results.ByBlockID(block.ID())
		if err != nil {
			return fmt.Errorf("failed to lookup execution result for block %d: %w", block.ID(), err)
		}

		exists, err := e.checkExecutionData(ctx, result.ExecutionDataID)

		if errors.Is(err, ErrRequesterHalted) {
			e.log.Error().Err(err).
				Str("block_id", block.ID().String()).
				Str("execution_data_id", result.ExecutionDataID.String()).
				Msg("HALTING REQUESTER: invalid execution data found")

			e.status.Halt(ctx)
			ctx.Throw(ErrRequesterHalted)
		}
		if err != nil {
			return err
		}

		if exists {
			// only track the state if this block needs a notification
			if height > e.status.lastNotified {
				e.status.Fetched(ctx, height, block.ID())
			}
			continue
		}

		// block until fetch is accepted
		e.fetchRetryRequests <- fetchRequest{
			blockID: block.ID(),
			height:  height,
		}
	}

	return nil
}

func (e *executionDataRequesterImpl) checkExecutionData(ctx irrecoverable.SignalerContext, rootID flow.Identifier) (bool, error) {
	// Has loads the first CID from disk to check if it exists. This means that the first CID from
	// every blob will be loaded twice. This is necessary to ensure we can distinguish between errors
	// returned from blobs that don't exist in the store, from ones that do. Since we don't accept
	// any newly downloaded blobs with MalformedDataError, getting one now for a blob that already
	// exists in the store indicates the data has been corrupted and should be refetched. This helps
	// make the store more resilient to partial disk failures, and data corruption without having to
	// refetch the entire set of blobs again.
	exists, err := e.eds.Has(ctx, rootID)

	if err != nil {
		// There was an unexpected error with the datastore
		return false, fmt.Errorf("error looking up data in blobstore: %w", err)
	}

	if !exists {
		return false, nil
	}

	// check that the data in the db is valid
	_, err = e.eds.Get(ctx, rootID)

	// The data was validated when it was originally stored, so any errors now about the data
	// validity indicate the data was modified on disk

	if errors.Is(err, &state_synchronization.BlobSizeLimitExceededError{}) {
		// This shouldn't be possible. It would mean that the data was updated in the db to
		// be well-formed but oversized.
		// Note: the criteria for this error cannot be made more strict, otherwise nodes may suddenly
		// fail to start up. If a stricter criteria is needed, we can add a new error type.
		e.log.Error().Err(err).
			Str("execution_data_id", rootID.String()).
			Msg("HALTING REQUESTER: invalid execution data found")

		ctx.Throw(ErrRequesterHalted)
	}

	if errors.Is(err, &state_synchronization.MalformedDataError{}) {
		// This is a special case where the data was corrupted on disk. Delete and refetch
		cid := flow.FlowIDToCid(rootID)
		e.bs.DeleteBlob(ctx, cid)
	}

	// Any errors at this point should be handled by refetching the data
	if err != nil {
		return false, nil
	}

	return true, nil
}

// Components:
// [x] Fetch EDs when new block is finalized
// [x] Handle Download failures
// [x] Handle missed blocks
// [x] Handle queue full
// [ ] Bootstrap node from empty DB mid-spork
// [ ] Bootstrap node with existing state
// [ ] Detect when notifications are blocked and recover
// [X] Ensure there is backpressure when downloads backup
// [X] Handle invalid data blobs gracefully
// [X] Don't refetch invalid data from network (avoid blob thrashing)

// [ ] Handle irrecoverable errors from this component without crashing the node
// [ ] Can we make the caching/status simpler?
