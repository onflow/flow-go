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
	"github.com/onflow/flow-go/module/irrecoverable"
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

// RequestExecutionData requests execution data for a block.
// It uses a retry mechanism to retry the download execution data if they are not found.
// Execution data are saved in the execution data cache passed on instantiation.
func (r *OneshotExecutionDataRequester) RequestExecutionData(
	ctx irrecoverable.SignalerContext,
	blockID flow.Identifier,
	height uint64, // TODO: do we even need it? we use it only for logging tho. and I'm not sure we have it in our case
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

		// don't retry if the blob was invalid
		if isInvalidBlobError(err) {
			return err
		}

		return retry.RetryableError(err)
	})
}

func (r *OneshotExecutionDataRequester) processFetchRequest(
	parentCtx irrecoverable.SignalerContext,
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

	logger.Info().
		Hex("execution_data_id", logging.ID(execData.ID())).
		Msg("execution data fetched")

	return nil
}
