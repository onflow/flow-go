package requester

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ExecutionDataRequester defines the interface for requesting execution data for a block.
type ExecutionDataRequester interface {
	// RequestExecutionData requests execution data for a given block.
	//
	// Expected errors:
	// - context.Canceled: if the provided context was canceled before completion
	// All other errors are unexpected exceptions and may indicate invalid execution data was received.
	RequestExecutionData(ctx context.Context) (*execution_data.BlockExecutionData, error)
}

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

var _ ExecutionDataRequester = (*OneshotExecutionDataRequester)(nil)

// OneshotExecutionDataRequester is a component that requests execution data for a block.
// It uses a retry mechanism to retry the download execution data if they are not found.
type OneshotExecutionDataRequester struct {
	log                zerolog.Logger
	metrics            module.ExecutionDataRequesterMetrics
	config             OneshotExecutionDataConfig
	execDataDownloader execution_data.Downloader
	executionResult    *flow.ExecutionResult
	blockHeader        *flow.Header
}

// NewOneshotExecutionDataRequester creates a new OneshotExecutionDataRequester instance.
// It validates that the provided block header and execution result are consistent
//
// Parameters:
//   - log: Logger instance for the requester component
//   - metrics: Metrics collector for execution data requester operations
//   - execDataDownloader: Cache for storing and retrieving execution data
//   - executionResult: The execution result to request data for
//   - blockHeader: The block header corresponding to the execution result
//   - config: Configuration settings for the oneshot execution data requester
//
// No errors are expected during normal operations and likely indicate a bug or
// inconsistent state.
func NewOneshotExecutionDataRequester(
	log zerolog.Logger,
	metrics module.ExecutionDataRequesterMetrics,
	execDataDownloader execution_data.Downloader,
	executionResult *flow.ExecutionResult,
	blockHeader *flow.Header,
	config OneshotExecutionDataConfig,
) (*OneshotExecutionDataRequester, error) {
	if blockHeader.ID() != executionResult.BlockID {
		return nil, fmt.Errorf("block id and execution result mismatch")
	}

	return &OneshotExecutionDataRequester{
		log:                log.With().Str("component", "oneshot_execution_data_requester").Logger(),
		metrics:            metrics,
		execDataDownloader: execDataDownloader,
		executionResult:    executionResult,
		blockHeader:        blockHeader,
		config:             config,
	}, nil
}

// RequestExecutionData requests execution data for a given block from the network.
// It performs a fetch using a retry mechanism with exponential backoff if execution data not found.
// Returns the execution data entity and any error encountered.
//
// Expected errors:
// - context.Canceled: if the provided context was canceled before completion
// All other errors are unexpected exceptions and may indicate invalid execution data was received.
func (r *OneshotExecutionDataRequester) RequestExecutionData(
	ctx context.Context,
) (*execution_data.BlockExecutionData, error) {
	backoff := retry.NewExponential(r.config.RetryDelay)
	backoff = retry.WithCappedDuration(r.config.MaxRetryDelay, backoff)
	backoff = retry.WithJitterPercent(15, backoff)

	// bitswap always waits for either all data to be received or a timeout, even if it encountered an error.
	// use an incremental backoff for the timeout so we do faster initial retries, then allow for more
	// time in case data is large or there is network congestion.
	timeout := retry.NewExponential(r.config.FetchTimeout)
	timeout = retry.WithCappedDuration(r.config.MaxFetchTimeout, timeout)

	attempt := 0
	lg := r.log.With().
		Str("block_id", r.executionResult.BlockID.String()).
		Str("execution_data_id", r.executionResult.ExecutionDataID.String()).
		Uint64("height", r.blockHeader.Height).
		Logger()

	var execData *execution_data.BlockExecutionData
	err := retry.Do(ctx, backoff, func(context.Context) error {
		if attempt > 0 {
			lg.Debug().
				Uint64("attempt", uint64(attempt)).
				Msgf("retrying download")

			r.metrics.FetchRetried()
		}
		attempt++

		// download execution data for the block
		fetchTimeout, _ := timeout.Next()
		var err error
		execData, err = r.processFetchRequest(ctx, fetchTimeout)
		if isBlobNotFoundError(err) || errors.Is(err, context.DeadlineExceeded) {
			return retry.RetryableError(err)
		}

		if execution_data.IsMalformedDataError(err) || execution_data.IsBlobSizeLimitExceededError(err) {
			// these errors indicate the execution data was received successfully and its hash matched
			// the value in the ExecutionResult, however, the data was malformed or invalid. this means that
			// an execution node produced an invalid execution data blob, and verification nodes approved it
			lg.Error().Err(err).
				Msg("received invalid execution data from network (potential slashing evidence?)")
		}

		return err
	})

	if err != nil {
		return nil, err
	}

	return execData, nil
}

// processFetchRequest performs the actual fetch of execution data for the given execution result.
//
// Expected errors during normal operations:
// - BlobNotFoundError if some CID in the blob tree could not be found from the blobstore
// - MalformedDataError if some level of the blob tree cannot be properly deserialized
// - BlobSizeLimitExceededError if some blob in the blob tree exceeds the maximum allowed size
// - context.DeadlineExceeded if fetching time exceeded fetchTimeout duration
// - context.Canceled if context was canceled during the request.
func (r *OneshotExecutionDataRequester) processFetchRequest(
	parentCtx context.Context,
	fetchTimeout time.Duration,
) (*execution_data.BlockExecutionData, error) {
	height := r.blockHeader.Height
	executionDataID := r.executionResult.ExecutionDataID

	lg := r.log.With().
		Str("block_id", r.executionResult.BlockID.String()).
		Str("execution_data_id", executionDataID.String()).
		Uint64("height", height).
		Logger()

	lg.Debug().Msg("processing fetch request")

	start := time.Now()
	r.metrics.ExecutionDataFetchStarted()
	lg.Debug().Msg("downloading execution data")

	ctx, cancel := context.WithTimeout(parentCtx, fetchTimeout)
	defer cancel()

	execData, err := r.execDataDownloader.Get(ctx, executionDataID)
	r.metrics.ExecutionDataFetchFinished(time.Since(start), err == nil, height)
	if err != nil {
		return nil, err
	}

	lg.Info().Msg("execution data fetched")

	return execData, nil
}
