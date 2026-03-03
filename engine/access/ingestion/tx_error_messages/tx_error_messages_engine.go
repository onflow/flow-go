package tx_error_messages

import (
	"context"
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
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	// processTxErrorMessagesWorkersCount defines the number of workers that
	// concurrently process transaction error messages in the job queue.
	processTxErrorMessagesWorkersCount = 3

	// defaultRetryDelay specifies the initial delay for the exponential backoff
	// when the process of fetching transaction error messages fails.
	//
	// This delay increases with each retry attempt, up to the maximum defined by
	// defaultMaxRetryDelay.
	defaultRetryDelay = 1 * time.Second

	// defaultMaxRetryDelay specifies the maximum delay for the exponential backoff
	// when the process of fetching transaction error messages fails.
	//
	// Once this delay is reached, the backoff will no longer increase with each retry.
	defaultMaxRetryDelay = 5 * time.Minute
)

// Engine represents the component responsible for managing and processing
// transaction result error messages. It retrieves, stores,
// and retries fetching of error messages from execution nodes, ensuring
// that they are processed and stored for sealed blocks.
//
// No errors are expected during normal operation.
type Engine struct {
	*component.ComponentManager

	log     zerolog.Logger
	metrics module.TransactionErrorMessagesMetrics
	state   protocol.State
	headers storage.Headers

	// Job queue
	txErrorMessagesConsumer *jobqueue.ComponentConsumer
	// Notifiers for queue consumer
	txErrorMessagesNotifier engine.Notifier

	txErrorMessagesCore *TxErrorMessagesCore // core logic for handling tx error messages
}

// New creates a new Engine instance, initializing all necessary components
// for processing transaction result error messages. This includes setting
// up the job queue and the notifier for handling finalized blocks.
//
// No errors are expected during normal operation.
func New(
	log zerolog.Logger,
	metrics module.TransactionErrorMessagesMetrics,
	state protocol.State,
	headers storage.Headers,
	txErrorMessagesProcessedHeight storage.ConsumerProgress,
	txErrorMessagesCore *TxErrorMessagesCore,
	finalizationRegistrar hotstuff.FinalizationRegistrar,
) (*Engine, error) {
	e := &Engine{
		log:                     log.With().Str("engine", "tx_error_messages_engine").Logger(),
		metrics:                 metrics,
		state:                   state,
		headers:                 headers,
		txErrorMessagesCore:     txErrorMessagesCore,
		txErrorMessagesNotifier: engine.NewNotifier(),
	}

	// jobqueue Jobs object that tracks sealed blocks by height. This is used by the txErrorMessagesConsumer
	// to get a sequential list of sealed blocks.
	sealedBlockReader := jobqueue.NewSealedBlockHeaderReader(state, headers)

	var err error
	// Create a job queue that will process error messages for new sealed blocks.
	// It listens to block finalization events from `txErrorMessagesNotifier`, then checks if there
	// are new sealed blocks with `sealedBlockReader`. If there are, it starts workers to process
	// them with `processTxResultErrorMessagesJob`, which fetches transaction error messages. At most
	// `processTxErrorMessagesWorkersCount` workers will be created for concurrent processing.
	// When a sealed block's error messages has been processed, it updates and persists the highest consecutive
	// processed height with `txErrorMessagesProcessedHeight`. That way, if the node crashes,
	// it reads the `txErrorMessagesProcessedHeight` and resume from `txErrorMessagesProcessedHeight + 1`.
	// If the database is empty, rootHeight will be used to init the last processed height.
	e.txErrorMessagesConsumer, err = jobqueue.NewComponentConsumer(
		e.log.With().Str("engine", "tx_error_messages").Logger(),
		e.txErrorMessagesNotifier.Channel(),
		txErrorMessagesProcessedHeight,
		sealedBlockReader,
		e.processTxResultErrorMessagesJob,
		processTxErrorMessagesWorkersCount,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating transaction result error messages jobqueue: %w", err)
	}

	e.metrics.TxErrorsInitialHeight(e.txErrorMessagesConsumer.LastProcessedIndex())

	// Add workers
	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.runTxResultErrorMessagesConsumer).
		Build()

	// register callback with finalization registrar
	finalizationRegistrar.AddOnBlockFinalizedConsumer(e.onFinalizedBlock)

	return e, nil
}

// processTxResultErrorMessagesJob processes a job for transaction error messages by
// converting the job to a block and processing error messages. If processing
// fails for all attempts, it logs the error.
func (e *Engine) processTxResultErrorMessagesJob(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
	header, err := jobqueue.JobToBlockHeader(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to block: %w", err))
	}

	start := time.Now()
	e.metrics.TxErrorsFetchStarted()

	err = e.processErrorMessagesForBlock(ctx, header.ID())

	// use the last processed index to ensure the metrics reflect the highest _consecutive_ height.
	// this makes it easier to see when downloading gets stuck at a height.
	e.metrics.TxErrorsFetchFinished(time.Since(start), err == nil, e.txErrorMessagesConsumer.LastProcessedIndex())

	if err == nil {
		done()
		return
	}

	e.log.Error().
		Err(err).
		Str("job_id", string(job.ID())).
		Msg("error encountered while processing transaction result error messages job")
}

// runTxResultErrorMessagesConsumer runs the txErrorMessagesConsumer component
func (e *Engine) runTxResultErrorMessagesConsumer(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.txErrorMessagesConsumer.Start(ctx)

	err := util.WaitClosed(ctx, e.txErrorMessagesConsumer.Ready())
	if err == nil {
		ready()

		// In the case where this component is started for the first time after a spork, we need to
		// manually trigger the first check since OnFinalizedBlock will never be called.
		// If this is started on a live network, an early check will be a no-op.
		e.txErrorMessagesNotifier.Notify()
	}

	<-e.txErrorMessagesConsumer.Done()
}

// onFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated.
// Receives block finalized events from the finalization registrar and forwards them to the txErrorMessagesConsumer.
func (e *Engine) onFinalizedBlock(*model.Block) {
	e.txErrorMessagesNotifier.Notify()
}

// processErrorMessagesForBlock processes transaction result error messages for block.
// If the process fails, it will retry, using exponential backoff.
//
// No errors are expected during normal operation.
func (e *Engine) processErrorMessagesForBlock(ctx context.Context, blockID flow.Identifier) error {
	backoff := retry.NewExponential(defaultRetryDelay)
	backoff = retry.WithCappedDuration(defaultMaxRetryDelay, backoff)
	backoff = retry.WithJitterPercent(15, backoff)

	attempt := 0
	return retry.Do(ctx, backoff, func(context.Context) error {
		if attempt > 0 {
			e.metrics.TxErrorsFetchRetried()
		}

		err := e.txErrorMessagesCore.FetchErrorMessages(ctx, blockID)
		if err != nil {
			e.log.Debug().
				Err(err).
				Str("block_id", blockID.String()).
				Uint64("attempt", uint64(attempt)).
				Msgf("failed to fetch transaction result error messages. will retry")
		}
		attempt++

		return retry.RetryableError(err)
	})
}
