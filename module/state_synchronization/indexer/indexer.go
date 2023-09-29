package indexer

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/storage"
)

const (
	workersCount = 1 // how many workers will concurrently process the tasks in the jobqueue
	searchAhead  = 1 // how many block heights ahead of the current will be requested and tasked for jobqueue
)

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
}

// NewIndexer creates a new execution worker.
func NewIndexer(
	log zerolog.Logger,
	initHeight uint64,
	fetchTimeout time.Duration,
	indexer *IndexerCore,
	executionCache *cache.ExecutionDataCache,
	executionDataLatestHeight func() (uint64, error),
	processedHeight storage.ConsumerProgress,
) *Indexer {
	r := &Indexer{
		exeDataNotifier: engine.NewNotifier(),
		indexer:         indexer,
	}

	r.exeDataReader = jobs.NewExecutionDataReader(executionCache, fetchTimeout, executionDataLatestHeight)

	// create a jobqueue that will process new available block execution data. The `exeDataNotifier` is used to
	// signal new work, which is being triggered on the `OnExecutionData` handler.
	r.Component = jobqueue.NewComponentConsumer(
		log.With().Str("module", "execution_indexer").Logger(),
		r.exeDataNotifier.Channel(),
		processedHeight,
		r.exeDataReader,
		initHeight,
		r.processExecutionData,
		workersCount,
		searchAhead,
	)

	return r
}

// Start the worker jobqueue to consume the available data.
func (i *Indexer) Start(ctx irrecoverable.SignalerContext) {
	i.exeDataReader.AddContext(ctx)
	i.Component.Start(ctx)
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
