package requester

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
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
// The requester is made up of 3 subcomponents:
//
// * OnBlockFinalized:     receives block finalized events from the finalization registrar and
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
//                         the registered consumers are guaranteed to receive each sealed block in
//                         consecutive height at least once.
//
//    +------------------+      +---------------+       +----------------------+
// -->| OnBlockFinalized |----->| blockConsumer |   +-->| notificationConsumer |
//    +------------------+      +-------+-------+   |   +-----------+----------+
//                                      |           |               |
//                               +------+------+    |        +------+------+
//                            xN | Worker Pool |----+     x1 | Worker Pool |----> Registered consumers
//                               +-------------+             +-------------+

const (
	// DefaultFetchTimeout is the default initial timeout for fetching ExecutionData from the
	// db/network. The timeout is increased using an incremental backoff until FetchTimeout.
	DefaultFetchTimeout = 10 * time.Second

	// DefaultMaxFetchTimeout is the default timeout for fetching ExecutionData from the db/network
	DefaultMaxFetchTimeout = 10 * time.Minute

	// DefaultRetryDelay is the default initial delay used in the exponential backoff for failed
	// ExecutionData download retries
	DefaultRetryDelay = 1 * time.Second

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

	// The initial timeout for fetching ExecutionData from the db/network
	FetchTimeout time.Duration

	// The max timeout for fetching ExecutionData from the db/network
	MaxFetchTimeout time.Duration

	// Exponential backoff settings for download retries
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
}

type executionDataRequester struct {
	component.Component
	downloader execution_data.Downloader
	metrics    module.ExecutionDataRequesterMetrics
	config     ExecutionDataConfig
	log        zerolog.Logger

	// Local db objects
	headers storage.Headers

	executionDataReader *jobs.ExecutionDataReader

	// Notifiers for queue consumers
	finalizationNotifier engine.Notifier

	// Job queues
	blockConsumer *jobqueue.ComponentConsumer

	execDataCache *cache.ExecutionDataCache
	distributor   *ExecutionDataDistributor
}

var _ state_synchronization.ExecutionDataRequester = (*executionDataRequester)(nil)

// New creates a new execution data requester component
func New(
	log zerolog.Logger,
	edrMetrics module.ExecutionDataRequesterMetrics,
	downloader execution_data.Downloader,
	execDataCache *cache.ExecutionDataCache,
	processedHeight storage.ConsumerProgressInitializer,
	processedNotifications storage.ConsumerProgressInitializer,
	state protocol.State,
	headers storage.Headers,
	cfg ExecutionDataConfig,
	distributor *ExecutionDataDistributor,
	finalizationRegistrar hotstuff.FinalizationRegistrar,
) (state_synchronization.ExecutionDataRequester, error) {
	e := &executionDataRequester{
		log:                  log.With().Str("component", "execution_data_requester").Logger(),
		downloader:           downloader,
		execDataCache:        execDataCache,
		metrics:              edrMetrics,
		headers:              headers,
		config:               cfg,
		finalizationNotifier: engine.NewNotifier(),
		distributor:          distributor,
	}

	// jobqueue Jobs object that tracks sealed blocks by height. This is used by the blockConsumer
	// to get a sequential list of sealed blocks.
	sealedBlockReader := jobqueue.NewSealedBlockHeaderReader(state, headers)

	// blockConsumer ensures every sealed block's execution data is downloaded.
	// It listens to block finalization events from `finalizationNotifier`, then checks if there
	// are new sealed blocks with `sealedBlockReader`. If there are, it starts workers to process
	// them with `processingBlockJob`, which fetches execution data. At most `fetchWorkers` workers
	// will be created for concurrent processing. When a sealed block's execution data has been
	// downloaded, it updates and persists the highest consecutive downloaded height with
	// `processedHeight`. That way, if the node crashes, it reads the `processedHeight` and resume
	// from `processedHeight + 1`. If the database is empty, rootHeight will be used to init the
	// last processed height. Once the execution data is fetched and stored, it notifies
	// `executionDataNotifier`.
	blockConsumer, err := jobqueue.NewComponentConsumer(
		e.log.With().Str("module", "block_consumer").Logger(),
		e.finalizationNotifier.Channel(), // to listen to finalization events to find newly sealed blocks
		processedHeight,                  // read and persist the downloaded height
		sealedBlockReader,                // read sealed blocks by height
		e.config.InitialBlockHeight,      // initial "last processed" height for empty db
		e.processBlockJob,                // process the sealed block job to download its execution data
		fetchWorkers,                     // the number of concurrent workers
		e.config.MaxSearchAhead,          // max number of unsent notifications to allow before pausing new fetches
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create block consumer: %w", err)
	}
	e.blockConsumer = blockConsumer

	// notifies distributor when new ExecutionData blobs are available
	// SetPostNotifier will notify executionDataNotifier AFTER e.blockConsumer.LastProcessedIndex is updated.
	e.blockConsumer.SetPostNotifier(func(module.JobID) {
		e.distributor.OnExecutionDataReceived()
	})

	e.metrics.ExecutionDataFetchFinished(0, true, e.blockConsumer.LastProcessedIndex())

	e.Component = component.NewComponentManagerBuilder().
		AddWorker(e.runBlockConsumer).
		Build()

	// register callback with finalization registrar
	finalizationRegistrar.AddOnBlockFinalizedConsumer(e.onBlockFinalized)

	return e, nil
}

// onBlockFinalized accepts block finalization notifications from the FollowerDistributor
func (e *executionDataRequester) onBlockFinalized(*model.Block) {
	e.finalizationNotifier.Notify()
}

// HighestConsecutiveHeight returns the highest consecutive block height for which ExecutionData
// has been received.
func (e *executionDataRequester) HighestConsecutiveHeight() uint64 {
	return e.blockConsumer.LastProcessedIndex()
}

// runBlockConsumer runs the blockConsumer component
func (e *executionDataRequester) runBlockConsumer(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	err := util.WaitClosed(ctx, e.downloader.Ready())
	if err != nil {
		return // context cancelled
	}

	e.blockConsumer.Start(ctx)

	err = util.WaitClosed(ctx, e.blockConsumer.Ready())
	if err == nil {
		ready()
	}

	<-e.blockConsumer.Done()
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
		return
	}

	// errors are thrown as irrecoverable errors except context cancellation, and invalid blobs
	// invalid blobs are logged, and never completed, which will halt downloads after maxSearchAhead
	// is reached.
	e.log.Fatal().Err(err).Str("job_id", string(job.ID())).
		Str("block_id", header.ID().String()).
		Msg("error encountered while processing block job")
}

// processSealedHeight downloads ExecutionData for the given block height.
// If the download fails, it will retry forever, using exponential backoff.
func (e *executionDataRequester) processSealedHeight(ctx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64) error {
	backoff := retry.NewExponential(e.config.RetryDelay)
	backoff = retry.WithCappedDuration(e.config.MaxRetryDelay, backoff)
	backoff = retry.WithJitterPercent(15, backoff)

	// bitswap always waits for either all data to be received or a timeout, even if it encountered an error.
	// use an incremental backoff for the timeout so we do faster initial retries, then allow for more
	// time in case data is large or there is network congestion.
	timeout := retry.NewExponential(e.config.FetchTimeout)
	timeout = retry.WithCappedDuration(e.config.MaxFetchTimeout, timeout)

	attempt := 0
	return retry.Do(ctx, backoff, func(context.Context) error {
		if attempt > 0 {
			e.log.Debug().
				Str("block_id", blockID.String()).
				Uint64("height", height).
				Uint64("attempt", uint64(attempt)).
				Msgf("retrying download")

			e.metrics.FetchRetried()
		}
		attempt++

		// download execution data for the block
		fetchTimeout, _ := timeout.Next()
		err := e.processFetchRequest(ctx, blockID, height, fetchTimeout)

		// don't retry if the blob was invalid
		if isInvalidBlobError(err) {
			return err
		}

		return retry.RetryableError(err)
	})
}

func (e *executionDataRequester) processFetchRequest(parentCtx irrecoverable.SignalerContext, blockID flow.Identifier, height uint64, fetchTimeout time.Duration) error {
	logger := e.log.With().
		Str("block_id", blockID.String()).
		Uint64("height", height).
		Logger()

	logger.Debug().Msg("processing fetch request")

	start := time.Now()
	e.metrics.ExecutionDataFetchStarted()

	logger.Debug().Msg("downloading execution data")

	ctx, cancel := context.WithTimeout(parentCtx, fetchTimeout)
	defer cancel()

	// This is a blocking call that waits until either all data is received, or timeout occurs.
	execData, err := e.execDataCache.ByBlockID(ctx, blockID)

	// use the last processed index to ensure the metrics reflect the highest _consecutive_ height.
	// this makes it easier to see when downloading gets stuck at a height.
	e.metrics.ExecutionDataFetchFinished(time.Since(start), err == nil, e.blockConsumer.LastProcessedIndex())

	if isInvalidBlobError(err) {
		// This means an execution result was sealed with an invalid execution data id (invalid data).
		// Eventually, verification nodes will verify that the execution data is valid, and not sign the receipt
		logger.Error().Err(err).Msg("HALTING REQUESTER: invalid execution data found")

		return err
	}

	// Some or all of the blob was missing or corrupt. retry
	if isBlobNotFoundError(err) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		logger.Error().Err(err).Msg("failed to get execution data for block")

		return err
	}

	// Any other error is unexpected
	if err != nil {
		logger.Error().Err(err).Msg("unexpected error fetching execution data")

		parentCtx.Throw(err)
	}

	logger.Debug().
		Hex("execution_data_id", logging.ID(execData.ExecutionDataID)).
		Msg("execution data fetched")

	return nil
}

// Notification Worker Methods

func isInvalidBlobError(err error) bool {
	var malformedDataError *execution_data.MalformedDataError
	var blobSizeLimitExceededError *execution_data.BlobSizeLimitExceededError
	return errors.As(err, &malformedDataError) ||
		errors.As(err, &blobSizeLimitExceededError)
}

func isBlobNotFoundError(err error) bool {
	var blobNotFoundError *execution_data.BlobNotFoundError
	return errors.As(err, &blobNotFoundError)
}
