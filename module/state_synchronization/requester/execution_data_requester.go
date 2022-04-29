package requester

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// The ExecutionDataRequester downloads ExecutionData for sealed blocks from other participants in
// the flow network. The ExecutionData for a sealed block should always downloadable, since a
// sealed block must have been executed.
//
// Once the ExecutionData for a block is downloaded, the node becomes a seeder for other participants
// on the network using the bitswap protocol. The downloading and seeding work is handled by the
// ExecutionDataService.
//
// The ExecutionDataRequester internally uses a job queue to request and download each sealed block
// with multiple workers. It downloads ExecutionData block by block towards the latest sealed block.
// In order to ensure it does not miss any sealed block to download, it persists the last downloaded
// height, and only increments it when the next height has been downloaded. In the event of a crash
// failure, it will read the last downloaded height, and process from the next un-downloaded height.
// The requester listens to block finalization event, and checks if sealed height has been changed,
// if changed, it create job for each un-downloaded and sealed height.
//
// The requester is made up of 4 subcomponents:
//
// * OnBlockFinalized:     receives block finalized events from the finalization distributor and
//                         forwards them to the blockConsumer.
//
// * blockConsumer:        is a jobqueue that receives block finalization events. On each event,
//                         it checks for the latest sealed block, then uses a pool of workers to
//                         download ExecutionData for each block from the network. After each
//                         successful download, the blockConsumer sends a notification to the
//                         notificationConsumer that a new ExecutionData is available.
//
// * notificationConsumer: is a jobqueue that receives ExecutionData fetched events. On each event,
//                         it checks if ExecutionData for the next consecutive block height is
//                         available, then uses a single worker to send notifications to registered
//                         consumers.
//
//    +------------------+      +---------------+       +----------------------+
// -->| OnBlockFinalized |----->| blockConsumer |   +-->| notificationConsumer |
//    +------------------+      +-------+-------+   |   +-----------+----------+
//                                      |           |               |
//                               +------+------+    |        +------+------+
//                            xN | Worker Pool |----+     x1 | Worker Pool |----> Registered consumers
//                               +-------------+             +-------------+

const (
	// DefaultFetchTimeout is the default timeout for fetching ExecutionData from the db/network
	DefaultFetchTimeout = 5 * time.Minute

	// DefaultRetryDelay is the default initial delay used in the exponential backoff for failed
	// ExecutionData download retries
	DefaultRetryDelay = 10 * time.Second

	// DefaultMaxRetryDelay is the default maximum delay used in the exponential backoff for failed
	// ExecutionData download retries
	DefaultMaxRetryDelay = 5 * time.Minute

	// DefaultMaxSearchAhead is the default max number of unsent notifications to allow before
	// pausing new fetches.
	DefaultMaxSearchAhead = 5000

	// Number of goroutines to use for downloading new ExecutionData from the network.
	fetchWorkers = 4
)

// ExecutionDataConfig contains configuration options for the ExecutionDataRequester
type ExecutionDataConfig struct {
	// The initial value to use as the last processed block height. This should be the
	// first block height to sync - 1
	InitialBlockHeight uint64

	// Max number of unsent notifications to allow before pausing new fetches. After exceeding this
	// limit, the requester will stop processing new finalized block notifications. This prevents
	// unbounded memory use by the requester if it gets stuck fetching a specific height.
	MaxSearchAhead uint64

	// The timeout for fetching ExecutionData from the db/network
	FetchTimeout time.Duration

	// Exponential backoff settings for download retries
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration

	// Whether or not to run datastore check on startup
	CheckEnabled bool
}

type executionDataRequester struct {
	component.Component
	cm      *component.ComponentManager
	ds      datastore.Batching
	bs      network.BlobService
	eds     state_synchronization.ExecutionDataService
	metrics module.ExecutionDataRequesterMetrics
	config  ExecutionDataConfig
	log     zerolog.Logger

	// Local db objects
	headers storage.Headers
	results storage.ExecutionResults

	executionDataReader *jobs.ExecutionDataReader

	// Notifiers for queue consumers
	finalizationNotifier engine.Notifier

	// Job queues
	blockConsumer        *jobqueue.ComponentConsumer
	notificationConsumer *jobqueue.ComponentConsumer

	// List of callbacks to call when ExecutionData is successfully fetched for a block
	consumers []state_synchronization.ExecutionDataReceivedCallback

	consumerMu sync.RWMutex
}

var _ state_synchronization.ExecutionDataRequester = (*executionDataRequester)(nil)

// New creates a new execution data requester component
func New(
	log zerolog.Logger,
	edrMetrics module.ExecutionDataRequesterMetrics,
	datastore datastore.Batching,
	blobservice network.BlobService,
	eds state_synchronization.ExecutionDataService,
	processedHeight storage.ConsumerProgress,
	processedNotifications storage.ConsumerProgress,
	state protocol.State,
	headers storage.Headers,
	results storage.ExecutionResults,
	cfg ExecutionDataConfig,
) (state_synchronization.ExecutionDataRequester, error) {
	e := &executionDataRequester{
		log:                  log.With().Str("component", "execution_data_requester").Logger(),
		ds:                   datastore,
		bs:                   blobservice,
		eds:                  eds,
		metrics:              edrMetrics,
		headers:              headers,
		results:              results,
		config:               cfg,
		finalizationNotifier: engine.NewNotifier(),
	}

	executionDataNotifier := engine.NewNotifier()

	// jobqueue Jobs object that tracks sealed blocks by height. This is used by the blockConsumer
	// to get a sequential list of sealed blocks.
	sealedBlockReader := jobqueue.NewSealedBlockHeaderReader(state, headers)

	// blockConsumer ensures every sealed block's execution data is downloaded.
	// It listens to block finalization events from `finalizationNotifier`, then checks
	// if there are new sealed blocks with `sealedBlockReader`.
	// If there are, it starts workers to process them with `processingBlockJob`, which fetches
	// execution data. At most `fetchWorkers` number of workers will be created for concurrent processing.
	// when a sealed block's execution data has been downloaded, it updates and persists
	// the highest consecutive downloaded height with `processedHeight`.
	// That way, if the node crashes, it reads the `processedHeight` and resume
	// from `processedHeight + 1`. If the database is empty, rootHeight will be used
	// to init the last processed height.
	// once the execution data is fetched and stored, it notifies `executionDataNotifier`.
	e.blockConsumer = jobqueue.NewComponentConsumer(
		log.With().Str("module", "block_consumer").Logger(),
		e.finalizationNotifier.Channel(), // to listen to finalization events to find newly sealed blocks
		processedHeight,                  // read and persist the downloaded height
		sealedBlockReader,                // read sealed blocks by height
		e.config.InitialBlockHeight,      // initial "last processed" height for empty db
		e.processBlockJob,                // process the sealed block job to download its execution data
		fetchWorkers,                     // the number of concurrent workers
		e.config.MaxSearchAhead,          // max number of unsent notifications to allow before pausing new fetches
	)
	// notifies notificationConsumer when new ExecutionData blobs are available
	e.blockConsumer.SetPostNotifier(func(module.JobID) { executionDataNotifier.Notify() })

	// jobqueue Jobs object tracks downloaded execution data by height. This is used by the
	// notificationConsumer to get downloaded execution data from storage.
	e.executionDataReader = jobs.NewExecutionDataReader(
		e.eds,
		e.headers,
		e.results,
		e.config.FetchTimeout,
		e.blockConsumer.LastProcessedIndex, // method to get the highest consecutive height to notify
	)

	// notificationConsumer consumes `OnExecutionDataFetched` events, and ensures its consumer
	// receives this event in consecutive block height order.
	// It listens to events from `executionDataNotifier`, which is delivered when
	// a block's execution data is downloaded and stored, and checks the `executionDataCache` to
	// find if the next un-processed consecutive height is available.
	// To know what's the height of the next un-processed consecutive height, it reads the latest
	// consecutive height in `processedNotifications`. And it's persisted in storage to be crash-resistant.
	// When a new consecutive height is available, it calls `processNotificationJob` to notify all the
	// `e.consumers`.
	// Note: the `e.consumers` will be guaranteed to receive at least one `OnExecutionDataFetched` event
	// for each sealed block in consecutive block height order.
	e.notificationConsumer = jobqueue.NewComponentConsumer(
		e.log.With().Str("module", "notification_consumer").Logger(),
		executionDataNotifier.Channel(), // listen for notifications from the block consumer
		processedNotifications,          // read and persist the notified height
		e.executionDataReader,           // read execution data by height
		e.config.InitialBlockHeight,     // initial "last processed" height for empty db
		e.processNotificationJob,        // process the job to send notifications for an execution data
		1,                               // use a single worker to ensure notification is delivered in consecutive order
		0,                               // search ahead limit controlled by worker count
	)

	builder := component.NewComponentManagerBuilder().
		AddWorker(e.runBootstrap).
		AddWorker(e.runBlockConsumer).
		AddWorker(e.runNotificationConsumer)

	e.cm = builder.Build()
	e.Component = e.cm

	return e, nil
}

// OnBlockFinalized accepts block finalization notifications from the FinalizationDistributor
func (e *executionDataRequester) OnBlockFinalized(*model.Block) {
	e.finalizationNotifier.Notify()
}

// AddOnExecutionDataFetchedConsumer adds a callback to be called when a new ExecutionData is received
// Callback Implementations must:
//   * be concurrency safe
//   * be non-blocking
//   * handle repetition of the same events (with some processing overhead).
func (e *executionDataRequester) AddOnExecutionDataFetchedConsumer(fn state_synchronization.ExecutionDataReceivedCallback) {
	e.consumerMu.Lock()
	defer e.consumerMu.Unlock()

	e.consumers = append(e.consumers, fn)
}

// runBootstrap runs the main process that processes finalized block notifications and requests
// ExecutionData for each block seal
func (e *executionDataRequester) runBootstrap(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	err := util.WaitClosed(ctx, e.eds.Ready())
	if err != nil {
		return // context cancelled
	}

	err = util.WaitClosed(ctx, e.notificationConsumer.Ready())
	if err != nil {
		return // context cancelled
	}

	// only run datastore check if enabled
	if e.config.CheckEnabled {
		err := e.checkDatastore(ctx)

		// Any error is unexpected and should crash the component
		if err != nil {
			ctx.Throw(err)
		}
	}

	ready()
}

// runBlockConsumer runs the blockConsumer component
func (e *executionDataRequester) runBlockConsumer(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	// start the blockConsumer after bootstrapping the requester is complete
	err := util.WaitClosed(ctx, e.Ready())
	if err != nil {
		return // context cancelled
	}

	e.blockConsumer.Start(ctx)

	// ignore context error since we always want to wait for Done()
	_ = util.WaitClosed(ctx, e.blockConsumer.Ready())

	<-e.blockConsumer.Done()
}

// runNotificationConsumer runs the notificationConsumer component
func (e *executionDataRequester) runNotificationConsumer(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.executionDataReader.AddContext(ctx)
	e.notificationConsumer.Start(ctx)

	err := util.WaitClosed(ctx, e.notificationConsumer.Ready())
	if err == nil {
		ready()
	}

	<-e.notificationConsumer.Done()
}

// Fetch Worker Methods

// processBlockJob consumes jobs from the blockConsumer and attempts to download an ExecutionData
// for the given block height.
func (e *executionDataRequester) processBlockJob(ctx irrecoverable.SignalerContext, job module.Job, jobComplete func()) {
	// convert job into a block entry
	header, err := jobqueue.JobToBlockHeader(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to block: %w", err))
	}

	err = e.processSealedHeight(ctx, header.ID(), header.Height)
	if err == nil {
		jobComplete()
	}

	// errors are thrown as irrecoverable errors except context cancellation, and invalid blobs
	// invalid blobs are logged, and never completed, which will halt downloads after maxSearchAhead
	// is reached.
	e.log.Err(err).Str("job_id", string(job.ID())).Msg("error encountered while processing block job")
}

// processSealedHeight downloads ExecutionData for the given block height.
// If the download fails, it will retry forever, using exponential backoff.
func (e *executionDataRequester) processSealedHeight(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
	backoff := retry.NewExponential(e.config.RetryDelay)
	backoff = retry.WithCappedDuration(e.config.MaxRetryDelay, backoff)
	backoff = retry.WithJitterPercent(15, backoff)

	attempt := 0
	return retry.Do(ctx, backoff, func(context.Context) error {
		if attempt > 0 {
			e.metrics.FetchRetried()
		}
		attempt++

		// download execution data for the block
		err := e.processFetchRequest(ctx, blockID, height)

		// don't retry if the blob was invalid
		if isInvalidBlobError(err) {
			return err
		}

		return retry.RetryableError(err)
	})
}

func (e *executionDataRequester) processFetchRequest(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
	logger := e.log.With().
		Str("block_id", blockID.String()).
		Uint64("height", height).
		Logger()

	logger.Debug().Msg("processing fetch request")

	result, err := e.results.ByBlockID(blockID)

	// By the time the block is sealed, the ExecutionResult must be in the db
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to lookup execution result for block: %w", err))
	}

	start := time.Now()
	e.metrics.ExecutionDataFetchStarted()

	_, err = e.fetchExecutionData(ctx, result.ExecutionDataID)

	e.metrics.ExecutionDataFetchFinished(time.Since(start), err == nil, height)

	if isInvalidBlobError(err) {
		// This means an execution result was sealed with an invalid execution data id (invalid data).
		// Eventually, verification nodes will verify that the execution data is valid, and not sign the receipt
		logger.Error().Err(err).
			Str("execution_data_id", result.ExecutionDataID.String()).
			Msg("HALTING REQUESTER: invalid execution data found")

		return err
	}

	// Some or all of the blob was missing or corrupt. retry
	if isBlobNotFoundError(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		logger.Error().Err(err).Msg("failed to get execution data for block")

		return err
	}

	// Any other error is unexpected
	if err != nil {
		logger.Error().Err(err).
			Str("execution_data_id", result.ExecutionDataID.String()).
			Msg("unexpected error fetching execution data")

		ctx.Throw(err)
	}

	logger.Debug().Msg("Fetched execution data")

	return nil
}

// fetchExecutionData fetches the ExecutionData by its ID, and times out if fetchTimeout is exceeded
func (e *executionDataRequester) fetchExecutionData(signalerCtx irrecoverable.SignalerContext, executionDataID flow.Identifier) (*state_synchronization.ExecutionData, error) {
	ctx, cancel := context.WithTimeout(signalerCtx, e.config.FetchTimeout)
	defer cancel()

	// Get the data from the network
	// this is a blocking call, won't be unblocked until either hitting error (including timeout) or
	// the data is received
	executionData, err := e.eds.Get(ctx, executionDataID)

	if err != nil {
		return nil, err
	}

	return executionData, nil
}

// Notification Worker Methods

func (e *executionDataRequester) processNotificationJob(ctx irrecoverable.SignalerContext, job module.Job, jobComplete func()) {
	// convert job into a block entry
	entry, err := jobs.JobToBlockEntry(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to entry: %w", err))
	}

	e.processNotification(ctx, entry.Height, entry.ExecutionData)
	jobComplete()
}

func (e *executionDataRequester) processNotification(ctx irrecoverable.SignalerContext, height uint64, executionData *state_synchronization.ExecutionData) {
	e.log.Debug().Msgf("notifying for block %d", height)

	// send notifications
	e.notifyConsumers(executionData)

	e.metrics.NotificationSent(height)
}

func (e *executionDataRequester) notifyConsumers(executionData *state_synchronization.ExecutionData) {
	e.consumerMu.RLock()
	defer e.consumerMu.RUnlock()

	for _, fn := range e.consumers {
		fn(executionData)
	}
}

// checkDatastore checks that valid ExecutionData exists in the datastore for all expected blocks
func (e *executionDataRequester) checkDatastore(parentCtx irrecoverable.SignalerContext) error {
	// branch a separate context so we can shutdown the local Execution Data Service when done
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	signalCtx, errChan := irrecoverable.WithSignaler(ctx)

	go func() {
		// rethrow any irrecoverable errors to requester's context
		if err := util.WaitError(errChan, ctx.Done()); err != nil {
			parentCtx.Throw(err)
		}
	}()

	eds := LocalExecutionDataService(signalCtx, e.ds, e.log)

	if err := util.WaitClosed(ctx, eds.Ready()); err != nil {
		return nil
	}

	checker := NewDatastoreChecker(
		e.log,
		e.bs,
		eds,
		e.headers,
		e.results,
		e.config.InitialBlockHeight,
		e.processSealedHeight,
	)

	err := checker.Run(signalCtx, e.notificationConsumer.LastProcessedIndex())

	// done with check, shutdown the local Execution Data Service
	cancel()
	<-eds.Done()

	return err
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
