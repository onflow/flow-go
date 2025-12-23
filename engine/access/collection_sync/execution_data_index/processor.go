package execution_data_index

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/collection_sync"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type ExecutionDataProcessor struct {
	component.Component
	log                     zerolog.Logger
	newExecutionDataIndexed engine.Notifier
	provider                collection_sync.ExecutionDataProvider
	indexer                 collection_sync.BlockCollectionIndexer
	// state
	processedHeight   *counters.PersistentStrictMonotonicCounter
	onIndexedCallback func(uint64)
}

var _ collection_sync.ExecutionDataProcessor = (*ExecutionDataProcessor)(nil)
var _ collection_sync.ProgressReader = (*ExecutionDataProcessor)(nil)
var _ component.Component = (*ExecutionDataProcessor)(nil)

func NewExecutionDataProcessor(
	log zerolog.Logger,
	provider collection_sync.ExecutionDataProvider,
	indexer collection_sync.BlockCollectionIndexer,
	processedHeight *counters.PersistentStrictMonotonicCounter,
	onIndexedCallback func(uint64),
) *ExecutionDataProcessor {
	edp := &ExecutionDataProcessor{
		log:                     log.With().Str("coll_sync", "data_processor").Logger(),
		newExecutionDataIndexed: engine.NewNotifier(),
		provider:                provider,
		indexer:                 indexer,
		processedHeight:         processedHeight,
		onIndexedCallback:       onIndexedCallback,
	}

	// Initialize the notifier so that even if no new execution data comes in,
	// the worker loop can still be triggered to process any existing data.
	edp.newExecutionDataIndexed.Notify()

	// Build component manager with worker loop
	cm := component.NewComponentManagerBuilder().
		AddWorker(edp.workerLoop).
		Build()

	edp.Component = cm

	return edp
}

func (edp *ExecutionDataProcessor) OnNewExectuionData() {
	edp.newExecutionDataIndexed.Notify()
}

// retryOnBlobNotFound executes the given function and retries it every 5 seconds
// if it returns a BlobNotFoundError. Each retry attempt is logged.
// Returns the result of the function call, or an error if retries fail or a non-BlobNotFoundError occurs.
func retryOnBlobNotFound(
	ctx irrecoverable.SignalerContext,
	log zerolog.Logger,
	height uint64,
	fn func() ([]*flow.Collection, error),
) ([]*flow.Collection, error) {
	collections, err := fn()
	if err == nil {
		return collections, nil
	}

	// If the error is not BlobNotFoundError, return immediately
	if !execution_data.IsBlobNotFoundError(err) {
		return nil, err
	}

	retryTicker := time.NewTicker(5 * time.Second)
	defer retryTicker.Stop()

	attempt := 1
	log.Error().
		Uint64("height", height).
		Err(err).
		Int("attempt", attempt).
		Msg("execution data not found, retrying every 5 seconds")

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-retryTicker.C:
			attempt++
			collections, err = fn()
			if err == nil {
				log.Info().
					Uint64("height", height).
					Int("attempt", attempt).
					Msg("successfully retrieved execution data after retry")
				return collections, nil
			}

			// If error is still BlobNotFoundError, continue retrying
			if execution_data.IsBlobNotFoundError(err) {
				log.Error().
					Uint64("height", height).
					Err(err).
					Int("attempt", attempt).
					Msg("execution data still not found, retrying")
				continue
			}

			// If error changed to something else, return it
			return nil, err
		}
	}
}

func (edp *ExecutionDataProcessor) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	// using a single threaded loop to index each execution for height
	// since indexing collections is blocking anyway, and reading the execution data
	// is quick, because we cache for 100 heights.
	for {
		select {
		case <-ctx.Done():
			return
		case <-edp.newExecutionDataIndexed.Channel():
			highestAvailableHeight := edp.provider.HighestIndexedHeight()
			lowestMissing := edp.processedHeight.Value() + 1

			for height := lowestMissing; height <= highestAvailableHeight; height++ {
				// TODO: This logic only supports ingesting execution data from sealed blocks. Once support is
				// added for syncing execution data for unsealed results, this logic will need to be updated
				// to account for execution forks.
				// Fetch execution data for this height. If the blob is not found (BlobNotFoundError),
				// retryOnBlobNotFound will automatically retry every 5 seconds until it's available or a different error occurs.
				// retry is not needed, because the provider is supposed to guarantee that all heights below
				// HighestIndexedHeight have execution data available.
				// this is for debugging purpose for now.
				collections, err := retryOnBlobNotFound(ctx, edp.log, height, func() ([]*flow.Collection, error) {
					return edp.provider.GetExecutionDataByHeight(ctx, height)
				})
				if err != nil {
					ctx.Throw(fmt.Errorf("collection_sync execution data processor: failed to get execution data for height %d: %w",
						height, err))
					return
				}

				// Note: the collections might have been indexed by fetcher engine already,
				// but IndexCollectionsForBlock will handle deduplication by first check if the collections already exist,
				// if so, it will skip indexing them again.
				err = edp.indexer.IndexCollectionsForBlock(height, collections)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to index collections for block height %d: %w", height, err))
					return
				}

				// Update processed height after successful indexing
				err = edp.processedHeight.Set(height)
				if err != nil {
					ctx.Throw(fmt.Errorf("failed to update processed height to %d: %w", height, err))
					return
				}

				// Log progress for each height with all relevant information
				edp.log.Debug().
					Uint64("indexed", height).
					Uint64("lowest_missing", lowestMissing).
					Uint64("highest_available", highestAvailableHeight).
					Uint64("processed_count", height-lowestMissing+1).
					Uint64("remaining_count", highestAvailableHeight-height).
					Uint64("total_to_process", highestAvailableHeight-lowestMissing+1).
					Msg("indexed execution data progress")

				edp.onIndexedCallback(height)
			}
		}
	}
}

// ProcessedHeight returns the highest consecutive height for which execution data has been processed,
// meaning the collections for that height have been indexed.
func (edp *ExecutionDataProcessor) ProcessedHeight() uint64 {
	return edp.processedHeight.Value()
}
