package indexer

import (
	"time"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"

	"github.com/onflow/flow-go/state/protocol"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/state_synchronization/requester/jobs"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

const workersCount = 4
const searchAhead = 1

type ExecutionDataWorker struct {
	log             zerolog.Logger
	exeDataReader   *jobs.ExecutionDataReader
	exeDataNotifier engine.Notifier
	consumer        *jobqueue.ComponentConsumer
	indexer         *ExecutionState
	state           protocol.State
}

func NewRequester(
	log zerolog.Logger,
	initHeight uint64,
	fetchTimeout time.Duration,
	executionCache *cache.ExecutionDataCache,
	processedHeight storage.ConsumerProgress,
) *ExecutionDataWorker {
	r := &ExecutionDataWorker{
		exeDataNotifier: engine.NewNotifier(),
	}

	// todo note: alternative would be to use the sealed header reader, and then in the worker actually fetch the execution data
	r.exeDataReader = jobs.NewExecutionDataReader(executionCache, fetchTimeout, r.highestConsecutiveHeight)

	r.consumer = jobqueue.NewComponentConsumer(
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

func (r *ExecutionDataWorker) highestConsecutiveHeight() (uint64, error) {
	head, err := r.state.Sealed().Head()
	if err != nil {
		return 0, err
	}
	return head.Height, nil
}

func (r *ExecutionDataWorker) OnExecutionData(_ *execution_data.BlockExecutionDataEntity) {
	r.exeDataNotifier.Notify()
}

func (r *ExecutionDataWorker) processExecutionData(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
	entry, err := jobs.JobToBlockEntry(job)
	if err != nil {
		r.log.Error().Err(err).Str("job_id", string(job.ID())).Msg("error converting execution data job")
		ctx.Throw(err)
	}

	err = r.indexer.IndexBlockData(entry.ExecutionData)
	if err != nil {
		r.log.Error().Err(err).Str("job_id", string(job.ID())).Msg("error during execution data index processing job")
		ctx.Throw(err)
	}

	done()
}
