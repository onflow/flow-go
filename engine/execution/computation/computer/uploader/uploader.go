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

// OnCompleteFunc is the type of function being called at upload completion.
type OnCompleteFunc func(*execution.ComputationResult, error)

func NewAsyncUploader(uploader Uploader,
	retryInitialTimeout time.Duration,
	maxRetryNumber uint64,
	log zerolog.Logger,
	metrics module.ExecutionMetrics) *AsyncUploader {
	return &AsyncUploader{
		unit:                engine.NewUnit(),
		uploader:            uploader,
		log:                 log.With().Str("component", "block_data_uploader").Logger(),
		metrics:             metrics,
		retryInitialTimeout: retryInitialTimeout,
		maxRetryNumber:      maxRetryNumber,
	}
}

// AsyncUploader wraps up another Uploader instance and make its upload asynchronous
type AsyncUploader struct {
	module.ReadyDoneAware
	unit                *engine.Unit
	uploader            Uploader
	log                 zerolog.Logger
	metrics             module.ExecutionMetrics
	retryInitialTimeout time.Duration
	maxRetryNumber      uint64
	onComplete          OnCompleteFunc // callback function called after Upload is completed
}

func (a *AsyncUploader) Ready() <-chan struct{} {
	return a.unit.Ready()
}

func (a *AsyncUploader) Done() <-chan struct{} {
	return a.unit.Done()
}

func (a *AsyncUploader) SetOnCompleteCallback(onComplete OnCompleteFunc) {
	a.onComplete = onComplete
}

func (a *AsyncUploader) Upload(computationResult *execution.ComputationResult) error {

	backoff := retry.NewFibonacci(a.retryInitialTimeout)
	backoff = retry.WithMaxRetries(a.maxRetryNumber, backoff)

	a.unit.Launch(func() {
		a.metrics.ExecutionBlockDataUploadStarted()
		start := time.Now()

		a.log.Debug().Msgf("computation result of block %s is being uploaded",
			computationResult.ExecutableBlock.ID().String())

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
		} else {
			a.log.Debug().Msgf("computation result of block %s was successfully uploaded",
				computationResult.ExecutableBlock.ID().String())
		}

		a.metrics.ExecutionBlockDataUploadFinished(time.Since(start))

		if a.onComplete != nil {
			a.onComplete(computationResult, err)
		}
	})
	return nil
}
