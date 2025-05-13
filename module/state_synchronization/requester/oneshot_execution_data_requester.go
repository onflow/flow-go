package requester

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/utils/logging"
)

// OneshotExecutionDataConfig is a config for the oneshot execution data requester.
// It contains the retry settings for the execution data fetch.
type OneshotExecutionDataConfig struct {
	// the initial timeout for fetching execution data from the db/network. The timeout is
	// increased using an incremental backoff until FetchTimeout.
	FetchTimeout time.Duration
	// the max timeout for fetching execution data from the db/network.
	MaxFetchTimeout time.Duration
	// the initial delay used in the exponential backoff for failed execution data download
	// retries.
	RetryDelay time.Duration
	// the max delay used in the exponential backoff for failed execution data download.
	MaxRetryDelay time.Duration
}

// OneshotExecutionDataRequester is a component that requests execution data for a block.
// It uses a retry mechanism to retry the download execution data if they are not found.
type OneshotExecutionDataRequester struct {
	log           zerolog.Logger
	metrics       module.ExecutionDataRequesterMetrics
	config        OneshotExecutionDataConfig
	execDataCache *cache.ExecutionDataCache
}

func NewOneshotExecutionDataRequester(
	log zerolog.Logger,
	metrics module.ExecutionDataRequesterMetrics,
	execDataCache *cache.ExecutionDataCache,
	config OneshotExecutionDataConfig,
) *OneshotExecutionDataRequester {
	return &OneshotExecutionDataRequester{
		log:           log.With().Str("component", "oneshot_execution_data_requester").Logger(),
		metrics:       metrics,
		execDataCache: execDataCache,
		config:        config,
	}
}

// RequestExecutionData requests execution data for a given block from the execution data cache.
// It performs a fetch using a retry mechanism with exponential backoff if execution data not found.
// Execution data are saved in the execution data cache passed on instantiation.
//
// The function logs each retry attempt and emits metrics for retries and fetch durations.
// The block height is used only for logging and metric purposes.
//
// Returns a terminal error if fetching fails due to non-retryable issues.
func (r *OneshotExecutionDataRequester) RequestExecutionData(
	ctx context.Context,
	blockID flow.Identifier,
	height uint64,
) error {
	backoff := retry.NewExponential(r.config.RetryDelay)
	backoff = retry.WithCappedDuration(r.config.MaxRetryDelay, backoff)
	backoff = retry.WithJitterPercent(15, backoff)

	// bitswap always waits for either all data to be received or a timeout, even if it encountered an error.
	// use an incremental backoff for the timeout so we do faster initial retries, then allow for more
	// time in case data is large or there is network congestion.
	timeout := retry.NewExponential(r.config.FetchTimeout)
	timeout = retry.WithCappedDuration(r.config.MaxFetchTimeout, timeout)

	attempt := 0
	return retry.Do(ctx, backoff, func(context.Context) error {
		if attempt > 0 {
			r.log.Debug().
				Str("block_id", blockID.String()).
				Uint64("height", height).
				Uint64("attempt", uint64(attempt)).
				Msgf("retrying download")

			r.metrics.FetchRetried()
		}
		attempt++

		// download execution data for the block
		fetchTimeout, _ := timeout.Next()
		err := r.processFetchRequest(ctx, blockID, height, fetchTimeout)
		if isBlobNotFoundError(err) || errors.Is(err, context.DeadlineExceeded) {
			return retry.RetryableError(err)
		}

		return err
	})
}

// processFetchRequest performs the actual fetch of execution data for the given block within the provided timeout.
//
// It wraps the fetch with metrics tracking and contextual logging. The execution data is retrieved
// from the execution data cache based on the block ID.
//
// Expected errors during normal operations:
// - storage.ErrNotFound if a seal or execution result is not available for the block
// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
// - BlobSizeLimitExceededError if some blob in the blob tree exceeds the maximum allowed size
// - context.DeadlineExceeded if fetching time exceeded fetchTimeout duration
func (r *OneshotExecutionDataRequester) processFetchRequest(
	parentCtx context.Context,
	blockID flow.Identifier,
	height uint64,
	fetchTimeout time.Duration,
) error {
	logger := r.log.With().
		Str("block_id", blockID.String()).
		Uint64("height", height).
		Logger()
	logger.Debug().Msg("processing fetch request")

	start := time.Now()
	r.metrics.ExecutionDataFetchStarted()
	logger.Debug().Msg("downloading execution data")

	ctx, cancel := context.WithTimeout(parentCtx, fetchTimeout)
	defer cancel()

	execData, err := r.execDataCache.ByBlockID(ctx, blockID)
	r.metrics.ExecutionDataFetchFinished(time.Since(start), err == nil, height)
	if err != nil {
		return err
	}

	logger.Info().
		Hex("execution_data_id", logging.ID(execData.ID())).
		Msg("execution data fetched")

	return nil
}
