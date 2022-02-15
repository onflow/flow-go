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

	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
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
	fetchTimeout = time.Minute

	// TODO: make these configurable?
	fetchWorkers            = 4
	executionDataCacheSize  = 50
	finalizationQueueLength = 500
	fetchQueueLength        = 500
)

var ErrRequesterHalted = errors.New("requester was halted due to invalid data")

// ExecutionDataRequester downloads ExecutionData for new blocks from the network using the
// ExecutionDataService. On startup, it checks that ExecutionData exists and is valid for all blocks
// since the configured root block.
// 2 priorities:
// * 1. download execution state to build a local index
// * 2. download and share execution state to maintain the sync protocol
// This means that we need to both keep as many blocks synced as possible, and notify consumers
// about new blocks in order. We don't want to delay syncing if we're

type ExecutionDataRequester interface {
	component.Component
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
	finalizedBlockQueue *fifoqueue.FifoQueue

	// fetchQueue accepts new fetch requests, which are consumed by a pool of fetch workers.
	fetchQueue *fifoqueue.FifoQueue

	// fetchRetryQueue accepts fetch retry requests, which are also consumed by the same pool of
	// fetch workers as fetchQueue, however fetchRetryQueue takes priority.
	fetchRetryQueue *fifoqueue.FifoQueue

	// Notifiers for queue consumers
	finalizedBlocksNotifier engine.Notifier
	fetchNotifier           engine.Notifier
	notificationNotifier    engine.Notifier

	cache             *executionDataCache
	notificationState *status
	consumerMu        sync.RWMutex

	// TODO: figure out how to deal with this situation. We want to drop the memory footprint and stop processing new blocks, but also not stop the component
	stop chan struct{}

	workerCount int
}

type fetchRequest struct {
	blockID flow.Identifier
	height  uint64
}

type ExecutionDataReceivedCallback func(*state_synchronization.ExecutionData)

// NewexecutionDataRequesterImpl creates a new execution data requester engine
func NewExecutionDataRequester(
	log zerolog.Logger,
	edsMetrics module.ExecutionDataServiceMetrics,
	finalizationDistributor *pubsub.FinalizationDistributor,
	datastore datastore.Batching,
	blobservice network.BlobService,
	eds state_synchronization.ExecutionDataService,
	rootBlock *flow.Block,
	blocks storage.Blocks,
	results storage.ExecutionResults,
) (ExecutionDataRequester, error) {

	var err error
	e := &executionDataRequesterImpl{
		log:       log.With().Str("component", "execution_data_requester").Logger(),
		ds:        datastore,
		bs:        blobservice,
		eds:       eds,
		rootBlock: rootBlock,
		blocks:    blocks,
		results:   results,
		stop:      make(chan struct{}),

		finalizedBlocksNotifier: engine.NewNotifier(),
		fetchNotifier:           engine.NewNotifier(),
		notificationNotifier:    engine.NewNotifier(),

		cache:             newExecutionDataCache(executionDataCacheSize),
		notificationState: &status{startHeight: rootBlock.Header.Height},
	}

	finalizationDistributor.AddOnBlockFinalizedConsumer(e.onBlockFinalized)

	e.finalizedBlockQueue, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(finalizationQueueLength),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for finalized blocks: %w", err)
	}

	e.fetchQueue, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(fetchQueueLength),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for fetch requests: %w", err)
	}

	e.fetchRetryQueue, err = fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(fetchQueueLength),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for refetch requests: %w", err)
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

func (e *executionDataRequesterImpl) onBlockFinalized(blockID flow.Identifier) {
	// stop accepting new blocks if the component is shutting down
	if util.CheckClosed(e.stop) || util.CheckClosed(e.cm.ShutdownSignal()) {
		e.log.Warn().Str("finalized_block_id", blockID.String()).Msg("ignoring finalized block. component is shutting down")
		return
	}

	e.log.Debug().Str("finalized_block_id", blockID.String()).Msg("received finalized block notification")

	accepted := e.finalizedBlockQueue.Push(blockID)
	if !accepted {
		// Silently drop notifications if the queue is full. The seals contained in the block will
		// be backfilled when the next finalized block is processed. This takes advantage of the
		// fact that seals are guranteed by hotstuff to be contiguous.
		e.log.Warn().Str("finalized_block_id", blockID.String()).Msg("finalized block queue is full")
		return
	}

	e.finalizedBlocksNotifier.Notify()
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

	// Keep the component alive after a halt if the component hasn't been shutdown yet.
	// Currently, ExecutionData IDs are included in ExecutionResults, but the value is not checked
	// by verification nodes. This means it's possible for an EN to generate an ExecutionResult with
	// an invalid ExecutionDataID, and for that result to be sealed. In that case, we want to stop
	// processing ExecutionData for new blocks since the data includes state diffs and must be
	// processed sequentially.
	// However, this condition should not cause the node to crash as that would result in all nodes
	// running this component to crash simultaneously. Pausing here keeps the component alive until
	// shutdown.
	<-ctx.Done()

	// TODO: is there any cleanup to do after a halt?
}

func (e *executionDataRequesterImpl) fetchRequestProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-e.eds.Ready()
	ready()

	e.fetchRequestProcessingLoop(ctx)
}

func (e *executionDataRequesterImpl) notificationProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	e.notificationProcessingLoop(ctx)
}

// finalizationProcessingLoop waits for finalized block notifications, then processes all available
// blocks in the queue
func (e *executionDataRequesterImpl) finalizationProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if util.CheckClosed(ctx.Done()) || util.CheckClosed(e.stop) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-e.stop:
			return
		case <-e.finalizedBlocksNotifier.Channel():
			e.processAvailableBlocks(ctx)
		}
	}
}

func (e *executionDataRequesterImpl) processAvailableBlocks(ctx irrecoverable.SignalerContext) {
	for {
		item, ok := e.finalizedBlockQueue.Pop()

		// return when queue is empty
		if !ok {
			return
		}

		blockID := item.(flow.Identifier)

		block, err := e.blocks.ByID(blockID)

		// block must be in the db, otherwise there's a problem with the state
		if err != nil {
			ctx.Throw(fmt.Errorf("failed to lookup finalized block %s in protocol state db: %w", blockID, err))
		}

		err = e.processBlockSeals(block)
		if err != nil {
			// all errors indicate a problem fetching data from the db
			ctx.Throw(fmt.Errorf("failed to process finalized block %s: %w", blockID, err))
		}
	}
}

// Will return an error if any of the seals processed are for a block that does not exist in the db
func (e *executionDataRequesterImpl) processBlockSeals(block *flow.Block) error {
	logger := e.log.With().Str("finalized_block_id", block.ID().String()).Logger()
	logger.Debug().Msgf("checking %d seals in block", len(block.Payload.Seals))

	if len(block.Payload.Seals) == 0 {
		return nil
	}

	// First, find all seals in the block and sort them by height (ascending)

	fetchRequests := []fetchRequest{}

	for _, seal := range block.Payload.Seals {
		sealedBlock, err := e.blocks.ByID(seal.BlockID)

		if err != nil {
			return fmt.Errorf("failed to lookup sealed block in protocol state db: %w", err)
		}

		logger.Debug().
			Str("sealed_block_id", seal.BlockID.String()).
			Uint64("sealed_block_height", sealedBlock.Header.Height).
			Msg("checking sealed block")

		fetchRequests = append(fetchRequests, fetchRequest{
			blockID: seal.BlockID,
			height:  sealedBlock.Header.Height,
		})
	}

	logger.Debug().Msgf("found %d seals in block", len(fetchRequests))

	sort.Slice(fetchRequests, func(i, j int) bool {
		return fetchRequests[i].height < fetchRequests[j].height
	})

	// Next check if there is a gap between the lowest sealed block height and the last processed
	// height, and record any that were missed. This allows us to catch up if we dropped some blocks
	// because the the queue filled

	last := e.notificationState.LastSealed()
	if last < fetchRequests[0].height-1 {
		missedHeights, err := e.missingBefore(last+1, fetchRequests[0].height)

		if err != nil {
			return fmt.Errorf("failed to lookup missing sealed block in protocol state db: %w", err)
		}

		fetchRequests = append(missedHeights, fetchRequests...)
	}

	// Finally, enqueue all blocks and notify workers

	highestSealed := last

	for _, request := range fetchRequests {
		logger.Debug().Msgf("enqueueing fetch request for block %d", request.height)

		accepted := e.fetchQueue.Push(request)
		if !accepted {
			// Silently drop requests if the queue is full. They will be backfilled when the next
			// finalized block is processed. This takes advantage of the fact that seals are
			// guranteed by hotstuff to be contiguous.
			logger.Warn().
				Str("sealed_block_id", request.blockID.String()).
				Uint64("sealed_block_height", request.height).
				Msg("fetch request queue is full")
			e.notificationState.Sealed(highestSealed)
			return nil
		}

		logger.Debug().Msgf("queue has %d requests", e.fetchQueue.Len())

		highestSealed = request.height
		e.fetchNotifier.Notify()
	}

	e.notificationState.Sealed(highestSealed)
	return nil
}

func (e *executionDataRequesterImpl) missingBefore(current uint64, end uint64) ([]fetchRequest, error) {
	missedHeights := []fetchRequest{}

	for current < end {
		e.log.Debug().Msgf("adding missed sealed height: %d, latest: %d", current, end)
		missedBlock, err := e.blocks.ByHeight(current)
		if err != nil {
			return nil, fmt.Errorf("failed to get block for height %d: %w", current, err)
		}

		missedHeights = append(missedHeights, fetchRequest{
			blockID: missedBlock.ID(),
			height:  current,
		})

		current++
	}

	return missedHeights, nil
}

// Fetch Worker Methods

func (e *executionDataRequesterImpl) fetchRequestProcessingLoop(ctx irrecoverable.SignalerContext) {

	e.consumerMu.Lock()
	workerID := e.workerCount
	e.workerCount++
	e.consumerMu.Unlock()

	logger := e.log.With().Int("worker_id", workerID).Logger()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stop:
			return
		case <-e.fetchNotifier.Channel():
			logger.Debug().Msgf("fetching available requests")
			e.processAvailableFetchRequests(ctx)
			logger.Debug().Msgf("done fetching requests")
		}
	}
}

func (e *executionDataRequesterImpl) processAvailableFetchRequests(ctx irrecoverable.SignalerContext) {

	e.log.Debug().Msg("processing fetch requests")
	for {
		// retries get priority since the seal height is most likely lower
		item, ok := e.fetchRetryQueue.Pop()
		if ok {
			e.processFetchRequest(ctx, item.(fetchRequest))
			continue
		}

		// Check for missing backfills

		item, ok = e.fetchQueue.Pop()
		if ok {
			e.processFetchRequest(ctx, item.(fetchRequest))
			continue
		}

		// return when queue is empty
		return
	}
}

func (e *executionDataRequesterImpl) processFetchRequest(ctx irrecoverable.SignalerContext, request fetchRequest) error {
	logger := e.log.With().Str("block_id", request.blockID.String()).Logger()

	logger.Debug().Msgf("processing fetch request for block %d", request.height)

	executionData, err := e.byBlockID(ctx, request.blockID)

	logger.Debug().Msgf("done processing fetch request for block %d", request.height)

	if errors.Is(err, &state_synchronization.MalformedDataError{}) || errors.Is(err, &state_synchronization.BlobSizeLimitExceededError{}) {
		// This means an execution result was sealed with an invalid execution data id (invalid data).
		// Eventually, verification nodes will verify that the execution data id is valid, and not sign the receipt
		// Crashing isn't a good idea since that would take down access to the network, but this should probably halt the network somehow
		// maybe we can flag it as bad and not reattempt

		// TODO: add metric
		logger.Error().Err(err).Msg("HALTING REQUESTER: invalid execution data found")
		close(e.stop)
	}

	if err != nil {
		logger.Error().Err(err).Msg("failed to get execution data for block")

		accepted := e.fetchRetryQueue.Push(request)
		if !accepted {
			logger.Warn().Msg("fetch retry queue is full")
			// TODO: do I need to track this somewhere?
			// will need a process that periodically re-queues missed requests
			// possible solution: when retry queue is empty and there is a gap between next notify and last processed, requeue blocks in between
			return nil
		}

		e.fetchNotifier.Notify()
		return nil
	}

	logger.Debug().Msgf("fetched execution data for height %d", request.height)

	logger.Debug().Msgf("fetch queue has %d requests", e.fetchQueue.Len())
	logger.Debug().Msgf("fetch retry queue has %d requests", e.fetchRetryQueue.Len())

	accepted := e.cache.Put(request.height, executionData)
	if !accepted {
		logger.Warn().Msg("execution data cache is full")
	}

	e.notificationState.Fetched(request.height, request.blockID)
	e.notificationNotifier.Notify()

	return nil
}

// byBlockID fetches the ExecutionData for a block by its ID
func (e *executionDataRequesterImpl) byBlockID(signalerCtx irrecoverable.SignalerContext, blockID flow.Identifier) (*state_synchronization.ExecutionData, error) {
	// The ExecutionResult contains the root CID for the block's execution data
	result, err := e.results.ByBlockID(blockID)
	if err != nil {
		signalerCtx.Throw(fmt.Errorf("failed to lookup execution result for block: %w", err))
	}

	ctx, cancel := context.WithTimeout(signalerCtx, fetchTimeout)
	defer cancel()

	// Fetch the ExecutionData for blockID from the blobstore. If it doesn't exist locally, it will
	// be fetched from the network.
	executionData, err := e.eds.Get(ctx, result.ExecutionDataID)

	if err != nil {
		return nil, fmt.Errorf("failed to get execution data for block: %w", err)
	}

	return executionData, nil
}

// Notification Worker Methods

func (e *executionDataRequesterImpl) notificationProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if util.CheckClosed(ctx.Done()) || util.CheckClosed(e.stop) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-e.stop:
			return
		case <-e.notificationNotifier.Channel():
			e.processAvailableNotifications(ctx)
		}
	}
}

func (e *executionDataRequesterImpl) processAvailableNotifications(ctx irrecoverable.SignalerContext) {
	logger := e.log.With().Str("process", "notifications").Logger()
	for {
		next, blockID, ok := e.notificationState.NextNotification()
		if !ok {
			logger.Debug().Msg("no more notifications to process")
			missing := e.notificationState.MissingHeights()
			logger.Debug().Msgf("%#v", missing)

			logger.Debug().Msgf("next should be %d", e.notificationState.lastNotified+1)
			return
		}

		logger.Debug().Msgf("notifying for block %d", next)

		executionData, ok := e.cache.Get(next)
		if !ok {
			logger.Debug().Msgf("execution data not in cache for block %d", next)
			// get it from disk
			var err error
			executionData, err = e.eds.Get(ctx, blockID)
			if err != nil {
				// At this point the data has been downloaded and validated, so it should be available
				// should this just delete the data and refetch?
				// TODO: handle this better
				ctx.Throw(fmt.Errorf("failed to get execution data for block: %w", err))
			}
		}

		// send notifications
		e.notifyConsumers(executionData)

		// update notification manifest that we've sent notifications for this block height, and cleanup the cache
		e.notificationState.Notified(next)
		e.cache.Delete(next)
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

	// TODO: if this is a fresh start, we'll need to provide the latest sealed block height as the
	// target to catch up to before starting the finalization loop

	return e.checkDatastore(ctx, e.notificationState.lastReceived)
}

func (e *executionDataRequesterImpl) loadSyncState(ctx irrecoverable.SignalerContext) error {

	// TODO: load this from disk

	err := e.notificationState.Load()
	if err != nil {
		e.log.Error().Err(err).Msg("failed to load notification state. using default")
		e.notificationState = &status{}
		// TODO: check if error is not found or something else
	}

	if e.notificationState.halted {
		e.log.Error().Msg("HALTING REQUESTER: requester was halted on a previous run due to invalid data")
		close(e.stop)
		return ErrRequesterHalted
	}

	// defaults to the start block when booting with a fresh db
	if e.notificationState.lastNotified == 0 {
		e.notificationState.lastNotified = e.rootBlock.Header.Height
		e.log.Debug().Msgf("setting lastNotified to root block height: %d", e.notificationState.lastNotified)
	}

	// at boot, we only have the start block and last block notifications were sent for,
	// then we scan through the db to find the other metrics.
	e.notificationState.lastReceived = e.notificationState.lastNotified
	e.notificationState.lastSealed = e.notificationState.lastNotified

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
			// TODO: does it make sense to crash? what happens if the lastReceived value in the db is just wrong?
			return fmt.Errorf("failed to get block for height %d: %w", height, err)
		}

		result, err := e.results.ByBlockID(block.ID())
		if err != nil {
			return fmt.Errorf("failed to lookup execution result for block %d: %w", block.ID(), err)
		}

		exists, err := e.checkExecutionData(ctx, result.ExecutionDataID)

		if errors.Is(err, ErrRequesterHalted) {
			e.log.Error().Err(err).Str("block_id", block.ID().String()).Msg("HALTING REQUESTER: invalid execution data found")
		}
		if err != nil {
			return err
		}

		if exists {
			// only track the state if this block needs a notification
			if height > e.notificationState.lastNotified {
				e.notificationState.Fetched(height, block.ID())
				e.notificationNotifier.Notify()
			}
			continue
		}

		// block until fetch is accepted
		for {
			request := fetchRequest{
				blockID: block.ID(),
				height:  height,
			}
			if accepted := e.fetchRetryQueue.Push(request); accepted {
				e.fetchNotifier.Notify()
				break
			}
			// TODO: don't use sleep
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

func (e *executionDataRequesterImpl) checkExecutionData(ctx irrecoverable.SignalerContext, rootID flow.Identifier) (bool, error) {
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
		// be well-formed but oversized
		close(e.stop)
		return true, ErrRequesterHalted
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

func (e *executionDataRequesterImpl) checkMissing(ctx irrecoverable.SignalerContext) error {
	// Checks if there are any missing blocks that aren't in the fetch queue

	// possible solution: when retry queue is empty and there is a gap between next notify and last processed, requeue blocks in between

	missing := e.notificationState.MissingHeights()

	if len(missing) == 0 {
		return nil
	}

	// TODO: how can this be made resilient to bugs so it can recover if it loses track of state?

	for _, height := range missing {
		block, err := e.blocks.ByHeight(height)
		if err != nil {
			return fmt.Errorf("failed to get block for height %d: %w", height, err)
		}

		// TODO: do I want to block here?
		// block until fetch is accepted
		for {
			request := fetchRequest{
				blockID: block.ID(),
				height:  height,
			}
			if accepted := e.fetchRetryQueue.Push(request); accepted {
				e.fetchNotifier.Notify()
				break
			}
			// TODO: don't use sleep
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// Components:
// [ ] Fetch EDs when new block is finalized
// [ ] Handle Download failures
// [ ] Handle missed blocks
// [ ] Handle queue full
// [ ] Bootstrap node from empty DB mid-spork
// [ ] Bootstrap node with existing state
// [ ] Detect when notifications are blocked and recover
// [ ] Ensure there is backpressure when downloads backup
// [ ] Handle invalid data blobs gracefully
// [ ] Don't refetch invalid data from network (avoid blob thrashing)

// BFT Attacks:
// [ ] Far future date (not possible when following seals)
// [ ] Invalid blob (make sure to only download it once)
// [ ]
// [ ]
// [ ]
// [ ]
