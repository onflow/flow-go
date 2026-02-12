package indexer

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

const (
	workersCount = 1 // how many workers will concurrently process the tasks in the jobqueue
	searchAhead  = 1 // how many block heights ahead of the current will be requested and tasked for jobqueue

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
// The processing of new available data is done by creating a jobqueue that uses the execution data reader to
// obtain new jobs. The worker also implements the `highestConsecutiveHeight` method which is used by the execution
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
	jobConsumer         *jobqueue.ComponentConsumer
	registers           storage.RegisterIndex
}

// NewIndexer creates a new execution worker.
func NewIndexer(
	log zerolog.Logger,
	initHeight uint64,
	registers storage.RegisterIndex,
	indexer *IndexerCore,
	executionCache *cache.ExecutionDataCache,
	executionDataLatestHeight func() (uint64, error),
	processedHeight storage.ConsumerProgress,
) (*Indexer, error) {
	lastProcessedHeight, err := processedHeight.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("could not get last processed height: %w", err)
	}

	r := &Indexer{
		log:                     log.With().Str("module", "execution_indexer").Logger(),
		exeDataNotifier:         engine.NewNotifier(),
		blockIndexedNotifier:    engine.NewNotifier(),
		lastProcessedHeight:     atomic.NewUint64(lastProcessedHeight),
		indexer:                 indexer,
		registers:               registers,
		ProcessedHeightRecorder: execution_data.NewProcessedHeightRecorderManager(initHeight),
	}

	r.exeDataReader = jobs.NewExecutionDataReader(executionCache, fetchTimeout, executionDataLatestHeight)

	// create a jobqueue that will process new available block execution data. The `exeDataNotifier` is used to
	// signal new work, which is being triggered on the `OnExecutionData` handler.
	jobConsumer, err := jobqueue.NewComponentConsumer(
		r.log,
		r.exeDataNotifier.Channel(),
		processedHeight,
		r.exeDataReader,
		r.processExecutionData,
		workersCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating execution data jobqueue: %w", err)
	}

	r.jobConsumer = jobConsumer

	// SetPostNotifier will notify blockIndexedNotifier AFTER e.jobConsumer.LastProcessedIndex is updated.
	r.jobConsumer.SetPostNotifier(func(module.JobID) {
		r.blockIndexedNotifier.Notify()
	})

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(r.runExecutionDataConsumer)
	cm.AddWorker(r.processBlockIndexed)
	r.Component = cm.Build()

	return r, nil
}

// runExecutionDataConsumer runs the jobConsumer component
//
// No errors are expected during normal operations.
func (i *Indexer) runExecutionDataConsumer(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	i.log.Info().Msg("starting execution data jobqueue")
	i.jobConsumer.Start(ctx)
	err := util.WaitClosed(ctx, i.jobConsumer.Ready())
	if err == nil {
		ready()
	}

	<-i.jobConsumer.Done()
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
	highestIndexedHeight := i.jobConsumer.LastProcessedIndex()

	if lastProcessedHeight < highestIndexedHeight {
		if lastProcessedHeight+1000 < highestIndexedHeight {
			i.log.Warn().Msgf("notifying processed heights from %d to %d", lastProcessedHeight+1, highestIndexedHeight)
		}
		// we need loop here because it's possible for a height to be missed here,
		// we should guarantee all heights are processed
		for height := lastProcessedHeight + 1; height <= highestIndexedHeight; height++ {
			// Use BlockIDByHeight instead of ByHeight since we only need to verify the block exists
			// and don't need the full header data. This avoids expensive header deserialization.
			// _, err := i.indexer.headers.BlockIDByHeight(height)
			// if err != nil {
			// 	// if the execution data is available, the block must be locally finalized
			// 	i.log.Error().Err(err).Msgf("could not get header for height %d:", height)
			// 	return fmt.Errorf("could not get header for height %d: %w", height, err)
			// }

			i.OnBlockProcessed(height)
		}
		i.lastProcessedHeight.Store(highestIndexedHeight)
	}

	return nil
}

// Start the worker jobqueue to consume the available data.
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
	select {
	case <-i.jobConsumer.Ready():
	default:
		// LastProcessedIndex is not meaningful until the component has completed startup
		return 0, fmt.Errorf("HighestIndexedHeight must not be called before the component is ready")
	}

	// The jobqueue maintains its own highest indexed height value, separate from the register db.
	// Since jobs are only marked complete when ALL data is indexed, the lastProcessedIndex must
	// be strictly less than or equal to the register db's LatestHeight.
	return i.jobConsumer.LastProcessedIndex(), nil
}

// OnExecutionData is used to notify when new execution data is downloaded by the execution data requester jobqueue.
func (i *Indexer) OnExecutionData(_ *execution_data.BlockExecutionDataEntity) {
	i.exeDataNotifier.Notify()
}

// processExecutionData is a worker method that is being called by the jobqueue when processing a new job.
// The job data contains execution data which we provide to the execution indexer to index it.
func (i *Indexer) processExecutionData(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
	entry, err := jobs.JobToBlockEntry(job)
	if err != nil {
		i.log.Error().Err(err).Str("job_id", string(job.ID())).Msg("error converting execution data job")
		ctx.Throw(err)
	}

	err = i.indexer.IndexBlockData(entry.ExecutionData)
	if err != nil {
		i.log.Error().Err(err).Str("job_id", string(job.ID())).Msg("error during execution data index processing job")
		ctx.Throw(err)
	}

	done()
}
