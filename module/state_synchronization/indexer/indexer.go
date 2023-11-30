package indexer

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
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

var _ state_synchronization.IndexReporter = (*Indexer)(nil)

// Indexer handles ingestion of new execution data available and uses the execution data indexer module
// to index the data.
// The processing of new available data is done by creating a jobqueue that uses the execution data reader to
// obtain new jobs. The worker also implements the `highestConsecutiveHeight` method which is used by the execution
// data reader, so it doesn't surpass the highest sealed block height when fetching the data.
// The execution state worker has a callback that is used by the upstream queues which download new execution data to
// notify new data is available and kick off indexing.
type Indexer struct {
	component.Component
	log             zerolog.Logger
	exeDataReader   *jobs.ExecutionDataReader
	exeDataNotifier engine.Notifier
	indexer         *IndexerCore
	jobConsumer     *jobqueue.ComponentConsumer
	registers       storage.RegisterIndex
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
	forceResetIndexerHeight bool,
) (*Indexer, error) {
	r := &Indexer{
		log:             log.With().Str("module", "execution_indexer").Logger(),
		exeDataNotifier: engine.NewNotifier(),
		indexer:         indexer,
		registers:       registers,
	}

	bootstrapped, err := r.validateStartState(processedHeight, forceResetIndexerHeight)
	if err != nil {
		return nil, fmt.Errorf("error validating start state: %w", err)
	}
	if forceResetIndexerHeight && bootstrapped {
		err = r.resetIndexedHeight(processedHeight)
		if err != nil {
			return nil, fmt.Errorf("error resetting indexed height: %w", err)
		}
	}

	r.exeDataReader = jobs.NewExecutionDataReader(executionCache, fetchTimeout, executionDataLatestHeight)

	// create a jobqueue that will process new available block execution data. The `exeDataNotifier` is used to
	// signal new work, which is being triggered on the `OnExecutionData` handler.
	jobConsumer, err := jobqueue.NewComponentConsumer(
		r.log,
		r.exeDataNotifier.Channel(),
		processedHeight,
		r.exeDataReader,
		initHeight,
		r.processExecutionData,
		workersCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating execution data jobqueue: %w", err)
	}

	r.jobConsumer = jobConsumer

	r.Component = r.jobConsumer

	return r, nil
}

// validateStartState validates that the start state of the register db and the indexer's jobqueue
// are in sync, and that if forceResetIndexerHeight
func (i *Indexer) validateStartState(processedHeight storage.ConsumerProgress, forceResetIndexerHeight bool) (bool, error) {
	lastHeight, err := processedHeight.ProcessedIndex()
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return false, fmt.Errorf("could not get indexed block height: %w", err)
		}

		// if no blocks have been indexed yet, then the register db must only have data from loading
		// the checkpoint.
		if i.registers.LatestHeight() != i.registers.FirstHeight() {
			return false, fmt.Errorf("indexed height is not set, but register db has indexed data")
		}

		// the indexer is uninitialized, and the register db has a single block from the checkpoint
		return false, nil
	}

	if forceResetIndexerHeight {
		// resetting the indexer's progress index is only supported when bootstrapping a new register db,
		// otherwise the register db will throw exceptions when indexing an already indexed height.
		if i.registers.LatestHeight() != i.registers.FirstHeight() {
			return false, fmt.Errorf("cannot reset indexed height, register db is already initialized")
		}

		return true, nil
	}

	// the jobqueue must either be the same as the last processed height or one block behind.
	// this accounts for the case where the node crashed after it finished indexing registers,
	// but before updating the processed height.
	if lastHeight > i.registers.LatestHeight() || lastHeight < i.registers.LatestHeight()-1 {
		return false, fmt.Errorf("indexed height %d does not match register db's latest height %d", lastHeight, i.registers.LatestHeight())
	}

	return true, nil
}

func (i *Indexer) resetIndexedHeight(processedHeight storage.ConsumerProgress) error {
	checkpointHeight := i.registers.FirstHeight()

	i.log.Info().Uint64("height", checkpointHeight).Msg("attempting to reset indexer start height")
	err := processedHeight.SetProcessedIndex(checkpointHeight)
	if err != nil {
		return fmt.Errorf("could not reset indexed block height: %w", err)
	}
	i.log.Info().Uint64("height", checkpointHeight).Msg("successfully reset indexer start height")

	return nil
}

// Start the worker jobqueue to consume the available data.
func (i *Indexer) Start(ctx irrecoverable.SignalerContext) {
	i.exeDataReader.AddContext(ctx)
	i.Component.Start(ctx)
}

// LowestIndexedHeight returns the lowest height indexed by the execution indexer.
func (i *Indexer) LowestIndexedHeight() uint64 {
	// TODO: use a separate value to track the lowest indexed height. We're using the registers db's
	// value here to start because it's convenient. When pruning support is added, this will need to
	// be updated.
	return i.registers.FirstHeight()
}

// HighestIndexedHeight returns the highest height indexed by the execution indexer.
func (i *Indexer) HighestIndexedHeight() uint64 {
	// The jobqueue maintains its own highest indexed height value, separate from the register db.
	// Since jobs are only marked complete when ALL data is indexed, the lastProcessedIndex must
	// be strictly less than or equal to the register db's LatestHeight.
	return i.jobConsumer.LastProcessedIndex()
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

	// validate state matches between the registers db and indexer
	if i.jobConsumer.LastProcessedIndex() != i.registers.LatestHeight() {
		ctx.Throw(fmt.Errorf("indexer height %d does not match register db's latest height %d",
			i.jobConsumer.LastProcessedIndex(),
			i.registers.LatestHeight()))
	}
}
