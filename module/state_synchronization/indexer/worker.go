package indexer

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const (
	workersCount = 1 // how many workers will concurrently process the tasks in the jobqueue
	searchAhead  = 1 // how many block heights ahead of the current will be requested and tasked for jobqueue
)

// ExecutionStateWorker handles ingestion of new execution data available and uses the execution data indexer module
// to index the data.
// The processing of new available data is done by creating a jobqueue that uses the execution data reader to
// obtain new jobs. The worker also implements the `highestConsecutiveHeight` method which is used by the execution
// data reader, so it doesn't surpass the highest sealed block height when fetching the data.
// The execution state worker has a callback that is used by the upstream queues which download new execution data to
// notify new data is available and kick off indexing.
type ExecutionStateWorker struct {
	*jobqueue.ComponentConsumer
	log             zerolog.Logger
	exeDataReader   *jobs.ExecutionDataReader
	exeDataNotifier engine.Notifier
	indexer         *ExecutionState
	state           protocol.State
}

// NewExecutionStateWorker creates a new execution worker.
func NewExecutionStateWorker(
	log zerolog.Logger,
	initHeight uint64,
	fetchTimeout time.Duration,
	indexer *ExecutionState,
	executionCache *cache.ExecutionDataCache,
	processedHeight storage.ConsumerProgress,
) *ExecutionStateWorker {
	r := &ExecutionStateWorker{
		exeDataNotifier: engine.NewNotifier(),
		indexer:         indexer,
	}

	// todo note: alternative would be to use the sealed header reader, and then in the worker actually fetch the execution data
	r.exeDataReader = jobs.NewExecutionDataReader(executionCache, fetchTimeout, r.highestConsecutiveHeight)

	// create a jobqueue that will process new available block execution data. The `exeDataNotifier` is used to
	// signal new work, which is being triggered on the `OnExecutionData` handler.
	r.ComponentConsumer = jobqueue.NewComponentConsumer(
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

// highestConsecutiveHeight uses protocol state database to query the latest available sealed block height, this
// method is being passed to the execution data reader as a limiter for latest height the reader is allowed to fetch.
func (r *ExecutionStateWorker) highestConsecutiveHeight() (uint64, error) {
	head, err := r.state.Sealed().Head() // use the one from the data requester
	if err != nil {
		return 0, err
	}
	return head.Height, nil
}

// OnExecutionData is used to notify when new execution data is downloaded by the execution data requester jobqueue.
func (r *ExecutionStateWorker) OnExecutionData(_ *execution_data.BlockExecutionDataEntity) {
	r.exeDataNotifier.Notify()
}

// processExecutionData is a worker method that is being called by the jobqueue when processing a new job.
// The job data contains execution data which we provide to the execution indexer to index it.
func (r *ExecutionStateWorker) processExecutionData(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
	entry, err := jobs.JobToBlockEntry(job)
	if err != nil {
		r.log.Error().Err(err).Str("job_id", string(job.ID())).Msg("error converting execution data job")
		ctx.Throw(err)
	}

	err = r.indexer.IndexBlockData(ctx, entry.ExecutionData)
	if err != nil {
		r.log.Error().Err(err).Str("job_id", string(job.ID())).Msg("error during execution data index processing job")
		ctx.Throw(err)
	}

	done()
}
