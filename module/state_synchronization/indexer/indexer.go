package indexer

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/storage"
)

const (
	// fetchTimeout is the timeout for retrieving execution data from the datastore
	// This is required by the execution data reader, but in practice, this isn't needed
	// here since the data is in a local db.
	fetchTimeout = 30 * time.Second
)

// ErrIndexNotInitialized is returned when the indexer is not initialized
//
// This generally indicates that the index databases are still being initialized, and trying again
// later may succeed
var ErrIndexNotInitialized = errors.New("index not initialized")

var _ state_synchronization.IndexReporter = (*Indexer)(nil)
var _ execution_data.ProcessedHeightRecorder = (*Indexer)(nil)

// Indexer handles ingestion of new execution data available and uses the execution data indexer module
// to index the data.
// The processing of new available data is done using a worker loop that processes execution data
// from the execution data reader. The worker also implements the `highestConsecutiveHeight` method which is used by the execution
// data reader, so it doesn't surpass the highest sealed block height when fetching the data.
// The execution state worker has a callback that is used by the upstream queues which download new execution data to
// notify new data is available and kick off indexing.
type Indexer struct {
	component.Component
	execution_data.ProcessedHeightRecorder

	log                  zerolog.Logger
	exeDataReader        *jobs.ExecutionDataReader
	exeDataNotifier      engine.Notifier
	blockIndexedNotifier engine.Notifier
	// lastProcessedHeight the last handled block height
	lastProcessedHeight *atomic.Uint64
	indexer             *IndexerCore
	registers           storage.RegisterIndex
}

// NewIndexer creates a new execution worker.
func NewIndexer(
	log zerolog.Logger,
	initHeight uint64,
	registers storage.RegisterIndex,
	indexer *IndexerCore,
	executionCache *cache.ExecutionDataCache,
	executionDataLatestHeight func() uint64,
	processedHeightInitializer storage.ConsumerProgressInitializer,
) (*Indexer, error) {
	r := &Indexer{
		log:                     log.With().Str("module", "execution_indexer").Logger(),
		exeDataNotifier:         engine.NewNotifier(),
		blockIndexedNotifier:    engine.NewNotifier(),
		lastProcessedHeight:     atomic.NewUint64(initHeight),
		indexer:                 indexer,
		registers:               registers,
		ProcessedHeightRecorder: execution_data.NewProcessedHeightRecorderManager(initHeight),
	}

	r.exeDataReader = jobs.NewExecutionDataReader(executionCache, fetchTimeout, executionDataLatestHeight)

	// Initialize the notifier so that even if no new execution data comes in,
	// the worker loop can still be triggered to process any existing data.
	r.exeDataNotifier.Notify()

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(r.workerLoop)
	cm.AddWorker(r.processBlockIndexed)
	r.Component = cm.Build()

	return r, nil
}

// workerLoop processes execution data in a single-threaded loop.
// It processes each height sequentially from the last processed height to the highest available height.
func (i *Indexer) workerLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-i.exeDataNotifier.Channel():
			highestAvailableHeight, err := i.exeDataReader.Head()
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to get highest available height: %w", err))
				return
			}

			lowestMissing := i.lastProcessedHeight.Load() + 1

			for height := lowestMissing; height <= highestAvailableHeight; height++ {
				job, err := i.exeDataReader.AtIndex(height)
				if err != nil {
					if errors.Is(err, storage.ErrNotFound) {
						// Height not available yet, break and wait for next notification
						break
					}
					ctx.Throw(fmt.Errorf("failed to get execution data for height %d: %w", height, err))
					return
				}

				entry, err := jobs.JobToBlockEntry(job)
				if err != nil {
					i.log.Error().Err(err).Uint64("height", height).Msg("error converting execution data job")
					ctx.Throw(fmt.Errorf("error converting execution data job for height %d: %w", height, err))
					return
				}

				err = i.indexer.IndexBlockData(entry.ExecutionData)
				if err != nil {
					i.log.Error().Err(err).Uint64("height", height).Msg("error during execution data index processing")
					ctx.Throw(fmt.Errorf("error during execution data index processing for height %d: %w", height, err))
					return
				}

				// Update last processed height after successful indexing
				i.lastProcessedHeight.Store(height)

				// Notify that a block has been indexed
				i.blockIndexedNotifier.Notify()
			}
		}
	}
}

// processBlockIndexed is a worker that processes indexed blocks.
func (i *Indexer) processBlockIndexed(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-i.blockIndexedNotifier.Channel():
			err := i.onBlockIndexed()
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// onBlockIndexed notifies ProcessedHeightRecorderManager that new block is indexed.
//
// Expected errors during normal operation:
//   - storage.ErrNotFound:  if no finalized block header is known at given height
func (i *Indexer) onBlockIndexed() error {
	lastProcessedHeight := i.lastProcessedHeight.Load()

	header, err := i.indexer.headers.ByHeight(lastProcessedHeight)
	if err != nil {
		// if the execution data is available, the block must be locally finalized
		i.log.Error().Err(err).Msgf("could not get header for height %d:", lastProcessedHeight)
		return fmt.Errorf("could not get header for height %d: %w", lastProcessedHeight, err)
	}

	i.OnBlockProcessed(header.Height)

	return nil
}

// Start starts the worker loop to process available execution data.
func (i *Indexer) Start(ctx irrecoverable.SignalerContext) {
	i.exeDataReader.AddContext(ctx)
	i.Component.Start(ctx)
}

// LowestIndexedHeight returns the lowest height indexed by the execution indexer.
func (i *Indexer) LowestIndexedHeight() (uint64, error) {
	// TODO: use a separate value to track the lowest indexed height. We're using the registers db's
	// value here to start because it's convenient. When pruning support is added, this will need to
	// be updated.
	return i.registers.FirstHeight(), nil
}

// HighestIndexedHeight returns the highest height indexed by the execution indexer.
func (i *Indexer) HighestIndexedHeight() (uint64, error) {
	// The lastProcessedHeight tracks the highest indexed height.
	// Since indexing is only marked complete when ALL data is indexed, the lastProcessedHeight must
	// be strictly less than or equal to the register db's LatestHeight.
	return i.lastProcessedHeight.Load(), nil
}

// OnExecutionData is used to notify when new execution data is downloaded by the execution data requester.
func (i *Indexer) OnExecutionData() {
	i.exeDataNotifier.Notify()
}
