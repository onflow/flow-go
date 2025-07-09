package uploader

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	a := &AsyncUploader{
		uploader:            uploader,
		log:                 log.With().Str("component", "block_data_uploader").Logger(),
		metrics:             metrics,
		retryInitialTimeout: retryInitialTimeout,
		maxRetryNumber:      maxRetryNumber,
		// we use a channel rather than a Fifoqueue here because a Fifoqueue might drop items when full,
		// but it is not acceptable to skip uploading an execution result
		queue: make(chan *execution.ComputationResult, 20000),
	}
	builder := component.NewComponentManagerBuilder()
	for i := 0; i < 10; i++ {
		builder.AddWorker(a.UploadWorker)
	}
	a.cm = builder.Build()
	a.Component = a.cm
	return a
}

// AsyncUploader wraps up another Uploader instance and make its upload asynchronous
type AsyncUploader struct {
	uploader            Uploader
	log                 zerolog.Logger
	metrics             module.ExecutionMetrics
	retryInitialTimeout time.Duration
	maxRetryNumber      uint64
	onComplete          OnCompleteFunc // callback function called after Upload is completed
	queue               chan *execution.ComputationResult
	cm                  *component.ComponentManager
	component.Component
}

// UploadWorker implements a component worker which asynchronously uploads computation results
// from the execution node (after a block is executed) to storage such as a GCP bucket or S3 bucket.
func (a *AsyncUploader) UploadWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	done := ctx.Done()
	for {
		select {
		case <-done:
			return
		case computationResult := <-a.queue:
			a.UploadTask(ctx, computationResult)
		}
	}
}

func (a *AsyncUploader) SetOnCompleteCallback(onComplete OnCompleteFunc) {
	a.onComplete = onComplete
}

// Upload adds the computation result to a queue to be processed asynchronously by workers,
// ensuring that multiple uploads can be run in parallel.
// No errors expected during normal operation.
func (a *AsyncUploader) Upload(computationResult *execution.ComputationResult) error {
	a.queue <- computationResult
	return nil
}

// UploadTask implements retrying for uploading computation results.
// When the upload is complete, the callback will be called with the result (for example,
// to record that the upload was successful) and any error.
// No errors expected during normal operation.
func (a *AsyncUploader) UploadTask(ctx context.Context, computationResult *execution.ComputationResult) {
	backoff := retry.NewFibonacci(a.retryInitialTimeout)
	backoff = retry.WithMaxRetries(a.maxRetryNumber, backoff)

	a.metrics.ExecutionBlockDataUploadStarted()
	start := time.Now()

	a.log.Debug().Msgf("computation result of block %s is being uploaded",
		computationResult.ExecutableBlock.ID().String())

	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		err := a.uploader.Upload(computationResult)
		if err != nil {
			a.log.Warn().Err(err).Msg("error while uploading block data, retrying")
		}
		return retry.RetryableError(err)
	})

	// We only log upload errors here because the errors originate from an external cloud provider
	// and the upload success is not critical to correct continued operation of the node
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
}
