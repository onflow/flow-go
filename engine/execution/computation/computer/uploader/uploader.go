package uploader

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/logging"

	"github.com/sethvargo/go-retry"
)

type Uploader interface {
	Upload(computationResult *execution.ComputationResult) error
}

func NewAsyncUploader(uploader Uploader, retryInitialTimeout time.Duration, maxRetryNumber uint64, log zerolog.Logger, metrics module.ExecutionMetrics) *AsyncUploader {
	return &AsyncUploader{
		unit:                engine.NewUnit(),
		uploader:            uploader,
		log:                 log.With().Str("component", "block_data_uploader").Logger(),
		metrics:             metrics,
		retryInitialTimeout: retryInitialTimeout,
		maxRetryNumber:      maxRetryNumber,
	}
}

type AsyncUploader struct {
	unit                *engine.Unit
	uploader            Uploader
	log                 zerolog.Logger
	metrics             module.ExecutionMetrics
	retryInitialTimeout time.Duration
	maxRetryNumber      uint64
}

func (a *AsyncUploader) Ready() <-chan struct{} {
	return a.unit.Ready()
}

func (a *AsyncUploader) Done() <-chan struct{} {
	return a.unit.Done()
}

func (a *AsyncUploader) Upload(computationResult *execution.ComputationResult) error {

	backoff := retry.NewFibonacci(a.retryInitialTimeout)
	backoff = retry.WithMaxRetries(a.maxRetryNumber, backoff)

	a.unit.Launch(func() {
		a.metrics.ExecutionBlockDataUploadStarted()
		start := time.Now()

		err := retry.Do(a.unit.Ctx(), backoff, func(ctx context.Context) error {
			err := a.uploader.Upload(computationResult)
			if err != nil {
				a.log.Warn().Err(err).Msg("error while uploading block data, retrying")
			}
			return retry.RetryableError(err)
		})

		if err != nil {
			a.log.Error().Err(err).
				Hex("block_id", logging.Entity(computationResult.ExecutableBlock)).
				Msg("failed to upload block data")
		}

		a.metrics.ExecutionBlockDataUploadFinished(time.Since(start))
	})
	return nil
}
