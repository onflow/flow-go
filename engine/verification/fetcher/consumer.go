package fetcher

import (
	"fmt"

	"github.com/rs/zerolog"

	chunkmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
)

const (
	DefaultJobIndex = int64(0)
)

// ChunkJob converts a Chunk into a Job to be used by job queue
type ChunkJob struct {
	ChunkLocator *chunkmodels.Locator
}

// ID converts chunk id into job id, which guarantees uniqueness
func (j *ChunkJob) ID() module.JobID {
	return chunkIDToJobID(j.ChunkLocator.ID())
}

func chunkIDToJobID(chunkID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", chunkID))
}

func ChunkLocatorToJob(locator *chunkmodels.Locator) *ChunkJob {
	return &ChunkJob{ChunkLocator: locator}
}

func JobToChunk(job storage.Job) *chunkmodels.Locator {
	chunkjob, _ := job.(*ChunkJob)
	return chunkjob.ChunkLocator
}

// ChunksJob wraps the storage layer to provide an abstraction for
// consumers to read jobs
type ChunksJob struct {
	chunks storage.ChunkQueue
}

func (j ChunksJob) AtIndex(index int64) (storage.Job, error) {
	chunk, err := j.chunks.AtIndex(index)
	if err != nil {
		return nil, fmt.Errorf("could not read chunk: %w", err)
	}
	return ChunkLocatorToJob(chunk), nil
}

type EngineWorker interface {
	ProcessMyChunk(locator *chunkmodels.Locator)
	WithFinishProcessing(finishProcessing FinishProcessing)
}

// Worker receives job from job consumer and converts it back to Chunk
// for engine to process
type Worker struct {
	engine   EngineWorker
	consumer *ChunkConsumer
}

func NewWorker(engine EngineWorker) *Worker {
	return &Worker{
		engine: engine,
	}
}

// Run converts the job to Chunk, it's guaranteed to work, because
// ChunksJob converted chunk into job symmetrically
func (w *Worker) Run(job storage.Job) {
	chunk := JobToChunk(job)
	w.engine.ProcessMyChunk(chunk)
}

func (w *Worker) FinishProcessing(chunkID flow.Identifier) {
	jobID := chunkIDToJobID(chunkID)
	w.consumer.FinishJob(jobID)
}

// FinishProcessing is for the worker's underneath engine to report a chunk
// has been processed without knowing the job queue
// it's a callback so that the worker can convert the chunk id into a job
// id, and notify the consumer about a finished job with the
type FinishProcessing interface {
	FinishProcessing(chunkID flow.Identifier)
}

// ChunkConsumer consumes the jobs from the job queue, and pass it to the
// Worker for processing.
// It wraps the generic job consumer in order to be used as a ReadyDoneAware
// on startup
type ChunkConsumer struct {
	consumer module.JobConsumer
}

func NewChunkConsumer(
	log zerolog.Logger,
	processedIndex storage.ConsumerProgress, // to persist the processed index
	chunksQueue storage.ChunkQueue, // to read jobs (chunks) from
	engine EngineWorker, // to process jobs (chunks)
	maxProcessing int64, // max number of jobs to be processed in parallel
	maxFinished int64, // when the next unprocessed job is not finished,
	// the max number of finished subsequent jobs before stopping processing more jobs
) *ChunkConsumer {
	worker := NewWorker(engine)
	engine.WithFinishProcessing(worker)

	jobs := &ChunksJob{chunks: chunksQueue}

	// TODO: adding meta to logger
	consumer := jobqueue.NewConsumer(
		log, jobs, processedIndex, worker, maxProcessing, maxFinished,
	)

	chunkConsumer := &ChunkConsumer{consumer}

	worker.consumer = chunkConsumer

	return chunkConsumer
}

func (c *ChunkConsumer) FinishJob(jobID module.JobID) {
	c.consumer.FinishJob(jobID)
}

func (c ChunkConsumer) Check() {
	c.consumer.Check()
}

func (c *ChunkConsumer) Ready() <-chan struct{} {
	err := c.consumer.Start(DefaultJobIndex)
	if err != nil {
		panic(fmt.Errorf("could not start the chunk consumer for match engine: %w", err))
	}

	ready := make(chan struct{})
	close(ready)
	return ready
}

func (c *ChunkConsumer) Done() <-chan struct{} {
	c.consumer.Stop()

	ready := make(chan struct{})
	close(ready)
	return ready
}
