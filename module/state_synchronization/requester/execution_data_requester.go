package requester

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/local"
	"github.com/onflow/flow-go/module/state_synchronization/requester/status"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/storage"
)

const (
	// DefaultFetchTimeout is the default timeout for fetching ExecutionData from the db/network
	DefaultFetchTimeout = 5 * time.Minute

	// DefaultMaxCachedEntries is the default the number of ExecutionData objects to keep when
	// waiting to send notifications. Dropped data is refetched from disk.
	DefaultMaxCachedEntries = 50

	// DefaultMaxSearchAhead is the default max number of unsent notifications to allow before
	// pausing new fetches. After exceeding this limit, the requester will stop processing new
	// finalized block notifications. This prevents unbounded memory use by the requester if it
	// gets stuck fetching a specific height.
	DefaultMaxSearchAhead = 5000

	// Number of goroutines to use for downloading new ExecutionData from the network.
	fetchWorkers = 4

	// Number of fetch requests to accept on the fetch queue before blocking
	// Note: this must be greater than fetchWorkers, otherwise the retry queue could overflow
	// resulting in lost retry requests
	fetchQueueLength = fetchWorkers * 2
)

// ErrRequesterHalted is returned when an invalid ExectutionData is encountered
var ErrRequesterHalted = errors.New("requester was halted due to invalid data")

// ExecutionDataReceivedCallback is a callback that is called ExecutionData is received for a new block
type ExecutionDataReceivedCallback func(*state_synchronization.ExecutionData)

// ExecutionDataRequester downloads ExecutionData for newly sealed blocks from the network using the
// ExecutionDataService. The requester has the following priorities:
//   1. ensure execution state is as widely distributed as possible among the network participants
//   2. make the execution state available to local subscribers
// The #1 priority of this component is to fetch ExecutionData for as many blocks as possible, making
// them available to other nodes in the network. This ensures execution state is available for all
// participants, and reduces network load on the execution nodes that source the data. The secondary
// priority is to consume the data locally.
type ExecutionDataRequester interface {
	component.Component
	OnBlockFinalized(*model.Block)
	AddOnExecutionDataFetchedConsumer(fn ExecutionDataReceivedCallback)
}

type executionDataRequesterImpl struct {
	component.Component
	cm      *component.ComponentManager
	ds      datastore.Batching
	bs      network.BlobService
	eds     state_synchronization.ExecutionDataService
	metrics module.ExecutionDataRequesterMetrics
	log     zerolog.Logger

	// Local db objects
	blocks  storage.Blocks
	results storage.ExecutionResults

	// The first block height for which to request ExecutionData
	rootBlockHeight uint64

	// List of callbacks to call when ExecutionData is successfully fetched for a block
	consumers []ExecutionDataReceivedCallback

	// finalizedBlockQueue accepts new finalized blocks to prevent blocking in the OnBlockFinalized
	// callback
	finalizedBlocks chan flow.Identifier

	// fetchQueue accepts new fetch requests, which are consumed by a pool of fetch workers.
	fetchRequests chan *status.BlockEntry

	// fetchRetryQueue accepts fetch retry requests, which are also consumed by the same pool of
	// fetch workers as fetchQueue, however fetchRetryQueue takes priority.
	fetchRetryRequests chan *status.BlockEntry

	// Notifiers for queue consumers
	notifications chan struct{}

	status     *status.Status
	consumerMu sync.RWMutex

	startupCheck bool
	fetchTimeout time.Duration

	bootstrapped chan struct{}
}

// NewexecutionDataRequester creates a new execution data requester component
func New(
	log zerolog.Logger,
	edrMetrics module.ExecutionDataRequesterMetrics,
	datastore datastore.Batching,
	blobservice network.BlobService,
	eds state_synchronization.ExecutionDataService,
	rootBlockHeight uint64,
	maxCachedEntries uint64,
	maxSearchAhead uint64,
	blocks storage.Blocks,
	results storage.ExecutionResults,
	fetchTimeout time.Duration,
	startupCheck bool,
) (ExecutionDataRequester, error) {
	e := &executionDataRequesterImpl{
		log:             log.With().Str("component", "execution_data_requester").Logger(),
		ds:              datastore,
		bs:              blobservice,
		eds:             eds,
		metrics:         edrMetrics,
		rootBlockHeight: rootBlockHeight,
		blocks:          blocks,
		results:         results,
		startupCheck:    startupCheck,
		fetchTimeout:    fetchTimeout,
		status: status.New(
			datastore,
			log.With().Str("component", "requester_status").Logger(),
			maxCachedEntries,
			maxSearchAhead,
		),

		finalizedBlocks:    make(chan flow.Identifier, 1),
		fetchRequests:      make(chan *status.BlockEntry, fetchQueueLength),
		fetchRetryRequests: make(chan *status.BlockEntry, fetchQueueLength),
		notifications:      make(chan struct{}, fetchQueueLength),
		bootstrapped:       make(chan struct{}),
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

func (e *executionDataRequesterImpl) OnBlockFinalized(block *model.Block) {
	logger := e.log.With().Str("finalized_block_id", block.BlockID.String()).Logger()

	// stop accepting new blocks if the component is shutting down
	if util.CheckClosed(e.cm.ShutdownSignal()) {
		logger.Warn().Msg("ignoring finalized block. component is shutting down")
		return
	}

	// limit how far ahead of the notifications we're allowed to get
	if e.status.IsPaused() {
		logger.Debug().Msg("ignoring finalized block. requester is paused")
		return
	}

	logger.Debug().Msg("received finalized block notification")

	select {
	case e.finalizedBlocks <- block.BlockID:
	default:
		logger.Warn().Msg("finalized block queue is full")
		e.metrics.FinalizationEventDropped()
	}
}

// finalizedBlockProcessor runs the main process that processes finalized block notifications and
// requests ExecutionData for each block seal
func (e *executionDataRequesterImpl) finalizedBlockProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	// Load previous requester state from db if it exists
	err := e.status.Load(ctx, e.rootBlockHeight)
	if err != nil {
		e.log.Error().Err(err).Msg("failed to load notification state. using defaults")
	}

	// Requester came up halted.
	if e.status.Halted() {
		e.log.Error().Msg("HALTING REQUESTER: requester was halted on a previous run due to invalid data")
		// By not closing the bootstrapped channel, none of the other workers will start, effectively
		// disabling the component.
		return
	}
	e.log.Debug().Msgf("starting with LastReceived: %d", e.status.LastReceived())

	close(e.bootstrapped)

	<-e.eds.Ready()

	// only run datastore check if enabled
	if e.startupCheck {
		err = e.checkDatastore(ctx, e.status.LastReceived())

		// Any error should crash the component
		if err != nil {
			ctx.Throw(err)
		}
	}

	ready()

	// Start ingesting new finalized block notifications
	e.finalizationProcessingLoop(ctx)
}

func (e *executionDataRequesterImpl) fetchRequestProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-e.bootstrapped
	<-e.eds.Ready()
	ready()

	e.fetchProcessingLoop(ctx)
}

func (e *executionDataRequesterImpl) notificationProcessor(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	// the notifier will use the ExecutionDataService to fetch blocks that aren't in the cache,
	// so it must be available
	<-e.bootstrapped
	<-e.eds.Ready()
	ready()

	e.notificationProcessingLoop(ctx)
}

// finalizationProcessingLoop waits for finalized block notifications, then processes all available
// blocks in the queue
func (e *executionDataRequesterImpl) finalizationProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if ctx.Err() != nil {
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

	// loop through all finalized blocks since the last processed block, and extract all seals
	lastHeight := e.status.LastProcessed()

	logger := e.log.With().Str("finalized_block_id", blockID.String()).Logger()
	logger.Debug().
		Uint64("start_height", lastHeight+1).
		Uint64("end_height", block.Header.Height).
		Msg("checking for seals")

	for height := lastHeight + 1; height <= block.Header.Height; height++ {
		if e.status.IsPaused() {
			logger.Debug().Uint64("height", height).Msg("pausing seal processing until workers catch up")
			break
		}

		logger.Debug().Uint64("height", height).Msg("processing height")
		err := e.processSealsFromHeight(ctx, height)

		if err != nil {
			ctx.Throw(fmt.Errorf("failed to process seals from height %d: %w", height, err))
		}

		e.status.Processed(height)
	}

	logger.Debug().Msg("done processing")
}

func (e *executionDataRequesterImpl) processSealsFromHeight(ctx irrecoverable.SignalerContext, height uint64) error {
	block, err := e.blocks.ByHeight(height)
	if err != nil {
		return fmt.Errorf("failed to get block: %w", err)
	}

	logger := e.log.With().
		Str("finalized_block_id", block.ID().String()).
		Uint64("finalized_block_height", height).
		Logger()

	if len(block.Payload.Seals) == 0 {
		logger.Debug().Msg("no seals in block")
		return nil
	}

	logger.Debug().Msgf("checking %d seals in block", len(block.Payload.Seals))

	// Find all seals in the block and sort them by height (ascending). This helps with processing
	// lower heights first since notifications are sent in order and seals are not guaranteed to
	// be sorted.

	requests, err := e.requestsFromSeals(block.Payload.Seals)
	if err != nil {
		return err
	}

	logger.Debug().Msgf("found %d seals in block", len(requests))

	sort.Slice(requests, func(i, j int) bool {
		return requests[i].Height < requests[j].Height
	})

	// Send all blocks to fetch workers (blocking to apply backpressure)
	for _, request := range requests {
		logger.Debug().Msgf("enqueueing fetch request for block %d", request.Height)

		select {
		case <-ctx.Done():
			return nil
		case e.fetchRequests <- request:
		}
	}

	return nil
}

func (e *executionDataRequesterImpl) requestsFromSeals(seals []*flow.Seal) ([]*status.BlockEntry, error) {
	requests := []*status.BlockEntry{}

	for _, seal := range seals {
		sealedBlock, err := e.blocks.ByID(seal.BlockID)

		if err != nil {
			return nil, fmt.Errorf("failed to lookup sealed block in protocol state db: %w", err)
		}

		requests = append(requests, &status.BlockEntry{
			BlockID: seal.BlockID,
			Height:  sealedBlock.Header.Height,
		})
	}

	return requests, nil
}

// Fetch Worker Methods

func (e *executionDataRequesterImpl) fetchProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if ctx.Err() != nil {
			return
		}

		select {
		case request := <-e.fetchRetryRequests:
			e.processFetchRequest(ctx, request)
			e.metrics.FetchRetried()
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

func (e *executionDataRequesterImpl) processFetchRequest(ctx irrecoverable.SignalerContext, request *status.BlockEntry) {
	logger := e.log.With().Str("block_id", request.BlockID.String()).Logger()
	logger.Debug().Msgf("processing fetch request for block %d", request.Height)

	result, err := e.results.ByBlockID(request.BlockID)

	// By the time the block is sealed, the ExecutionResult must be in the db
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to lookup execution result for block: %w", err))
	}

	executionData, err := e.fetchExecutionData(ctx, result.ExecutionDataID, request.Height)

	if isInvalidBlobError(err) {
		// This means an execution result was sealed with an invalid execution data id (invalid data).
		// Eventually, verification nodes will verify that the execution data is valid, and not sign the receipt
		logger.Error().Err(err).
			Str("execution_data_id", result.ExecutionDataID.String()).
			Msg("HALTING REQUESTER: invalid execution data found")

		e.status.Halt(ctx)
		e.metrics.Halted()
		ctx.Throw(ErrRequesterHalted)
	}

	// Some or all of the blob was missing or corrupt. retry
	if isBlobNotFoundError(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		logger.Error().Err(err).Msg("failed to get execution data for block")

		select {
		case e.fetchRetryRequests <- request:
		default:
			// We can't block here otherwise we risk a deadlock. However, since the retry queue is
			// always checked first, there can be at most fetchWorkers outstanding retry requests.
			// In practice, this situation can only happen if the retry channel's buffer size is
			// misconfigured.
			logger.Error().
				Str("execution_data_id", result.ExecutionDataID.String()).
				Msg("fetch retry queue is full")

			e.metrics.RetryDropped()
		}
		return
	}

	// Any other error is unexpected
	if err != nil {
		logger.Error().Err(err).
			Str("execution_data_id", result.ExecutionDataID.String()).
			Msg("unexpected error fetching execution data")

		ctx.Throw(err)
	}

	logger.Debug().Msgf("Fetched execution data for block %d", request.Height)

	request.ExecutionData = executionData

	e.status.Fetched(ctx, request)

	select {
	case <-ctx.Done():
		return
	case e.notifications <- struct{}{}:
	}
}

// fetchExecutionData fetches the ExecutionData by its ID, using fetchTimeout
func (e *executionDataRequesterImpl) fetchExecutionData(signalerCtx irrecoverable.SignalerContext, executionDataID flow.Identifier, height uint64) (executionData *state_synchronization.ExecutionData, err error) {
	ctx, cancel := context.WithTimeout(signalerCtx, e.fetchTimeout)
	defer cancel()

	start := time.Now()
	e.metrics.ExecutionDataFetchStarted()
	defer func() {
		// needs to be run inside a closure so the variables are resolved when the defer is executed
		e.metrics.ExecutionDataFetchFinished(time.Since(start), err == nil, height)
	}()

	// Get the data from the network
	executionData, err = e.eds.Get(ctx, executionDataID)

	if err != nil {
		return nil, err
	}

	// Write it to the local blobstore
	_, _, err = e.eds.Add(signalerCtx, executionData)

	if err != nil {
		return nil, fmt.Errorf("failed to write execution data to blobstore: %w", err)
	}

	return executionData, nil
}

// Notification Worker Methods

func (e *executionDataRequesterImpl) notificationProcessingLoop(ctx irrecoverable.SignalerContext) {
	for {
		// prioritize shutdowns
		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-e.notifications:
			e.sendNotifications(ctx)
		}
	}
}

func (e *executionDataRequesterImpl) sendNotifications(ctx irrecoverable.SignalerContext) {
	for {
		entry, ok := e.status.NextNotification()

		// we haven't finished fetching the next block to notify
		if !ok {
			e.log.Debug().Msgf("waiting to notify for block %d", e.status.NextNotificationHeight())
			return
		}

		e.log.Debug().Msgf("notifying for block %d", entry.Height)
		e.metrics.NotificationSent(entry.Height)

		// ExecutionData may have been purged, in which case, look it up again
		if entry.ExecutionData == nil {
			e.log.Debug().Msgf("execution data not in cache for block %d", entry.Height)

			// get it from db
			result, err := e.results.ByBlockID(entry.BlockID)
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to lookup execution result for block: %w", err))
			}

			entry.ExecutionData, err = e.eds.Get(ctx, result.ExecutionDataID)

			if err != nil {
				// At this point the data has been downloaded and validated, so it should be available
				ctx.Throw(fmt.Errorf("failed to get execution data for block: %w", err))
			}
		}

		// send notifications
		e.notifyConsumers(entry.ExecutionData)
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

func (e *executionDataRequesterImpl) checkDatastore(ctx irrecoverable.SignalerContext, lastReceivedHeight uint64) error {
	// we're only interested in inspecting blobs that exist in our local db, so create an
	// ExecutionDataService that only uses the local datastore
	localEDS, err := e.localExecutionDataService(ctx)

	if errors.Is(err, context.Canceled) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to create local ExecutionDataService: %w", err)
	}

	// Search from genesis to the lastReceivedHeight, and confirm data is still available for all heights
	// Update the notification state based on the data in the db
	for height := e.rootBlockHeight; height <= lastReceivedHeight; height++ {
		if ctx.Err() != nil {
			return nil
		}

		block, err := e.blocks.ByHeight(height)
		if err != nil {
			return fmt.Errorf("failed to get block for height %d: %w", height, err)
		}

		result, err := e.results.ByBlockID(block.ID())
		if err != nil {
			return fmt.Errorf("failed to lookup execution result for block %d: %w", block.ID(), err)
		}

		exists, err := e.checkExecutionData(ctx, localEDS, result.ExecutionDataID)

		if errors.Is(err, ErrRequesterHalted) {
			e.log.Error().Err(err).
				Str("block_id", block.ID().String()).
				Str("execution_data_id", result.ExecutionDataID.String()).
				Msg("HALTING REQUESTER: invalid execution data found")

			e.status.Halt(ctx)
			e.metrics.Halted()
			ctx.Throw(ErrRequesterHalted)
		}

		if err != nil {
			return err
		}

		if !exists {
			// block until fetch is accepted
			e.fetchRequests <- &status.BlockEntry{
				BlockID: block.ID(),
				Height:  height,
			}
		}
	}

	return nil
}

func (e *executionDataRequesterImpl) checkExecutionData(ctx irrecoverable.SignalerContext, localEDS state_synchronization.ExecutionDataService, rootID flow.Identifier) (bool, error) {
	invalidCIDs, cidErrs := localEDS.Check(ctx, rootID)

	// Check returns a list of CIDs with the corresponding errors encountered while retrieving their
	// data from the local datastore.

	if len(invalidCIDs) == 0 {
		return true, nil
	}

	// Track if this blob should be refetched
	missing := false

	var errs *multierror.Error
	for i, cid := range invalidCIDs {
		err := cidErrs[i]

		// Not Found, just report and continue
		if errors.Is(err, blockservice.ErrNotFound) {
			missing = true
			continue
		}

		// The blob's hash didn't match. This is a special case where the data was corrupted on
		// disk. Delete and refetch
		if errors.Is(err, blockstore.ErrHashMismatch) {
			if err := e.bs.DeleteBlob(ctx, cid); err != nil {
				return false, fmt.Errorf("failed to delete corrupted CID %s from rootCID %s: %w", cid, rootID, err)
			}

			missing = true
			continue
		}

		// It should not be possible to encounter one of these errors since they are checked when
		// the block is originally received
		if isInvalidBlobError(err) {
			return false, ErrRequesterHalted
		}

		// Record any other errors to return
		errs = multierror.Append(errs, err)
	}

	return missing, errs.ErrorOrNil()
}

// localExecutionDataService returns an ExecutionDataService that's configured to use only the local
// datastore, and to rehash blobs on read. This is used to check the validity of blobs that exist in
// the local db.
func (e *executionDataRequesterImpl) localExecutionDataService(ctx irrecoverable.SignalerContext) (state_synchronization.ExecutionDataService, error) {
	blobService := local.NewBlobService(e.ds, local.WithHashOnRead(true))
	blobService.Start(ctx)

	eds := state_synchronization.NewExecutionDataService(
		new(cbor.Codec),
		compressor.NewLz4Compressor(),
		local.NewBlobService(e.ds),
		metrics.NewNoopCollector(),
		e.log,
	)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-eds.Ready():
	}

	return eds, nil
}

func isInvalidBlobError(err error) bool {
	var malformedDataError *state_synchronization.MalformedDataError
	var blobSizeLimitExceededError *state_synchronization.BlobSizeLimitExceededError
	return errors.As(err, &malformedDataError) ||
		errors.As(err, &blobSizeLimitExceededError) ||
		errors.Is(err, state_synchronization.ErrBlobTreeDepthExceeded)
}

func isBlobNotFoundError(err error) bool {
	var blobNotFoundError *state_synchronization.BlobNotFoundError
	return errors.As(err, &blobNotFoundError)
}

// [ ] Add metrics
// * Fetches in progress
// * Fetch duration
// * Blocks behind latest
// * Outstanding notifications
// * Outstanding fetches
// * Dropped finalization messages
// * Dropped retry requests
// * Failed downloads
// * most highest notified height
// * most highest fetched height
// * heap size
