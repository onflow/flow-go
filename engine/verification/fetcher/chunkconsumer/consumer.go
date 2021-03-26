package fetcher

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
)

const (
	DefaultJobIndex = uint64(0)
)

// ChunkJob converts a chunk locator into a Job to be used by job queue.
type ChunkJob struct {
	ChunkLocator *chunks.Locator
}

// ID converts chunk locator identifier into job id, which guarantees uniqueness.
func (j ChunkJob) ID() module.JobID {
	return locatorIDToJobID(j.ChunkLocator.ID())
}

func locatorIDToJobID(locatorID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", locatorID))
}

func ChunkLocatorToJob(locator *chunks.Locator) *ChunkJob {
	return &ChunkJob{ChunkLocator: locator}
}

func JobToChunkLocator(job module.Job) (*chunks.Locator, error) {
	chunkjob, ok := job.(*ChunkJob)
	if !ok {
		return nil, fmt.Errorf("could not assert job to chunk locator, job id: %x", job.ID())
	}
	return chunkjob.ChunkLocator, nil
}

// ChunkJobs wraps the storage layer to provide an abstraction for consumers to read jobs.
type ChunkJobs struct {
	locators storage.ChunksQueue
}

func (j *ChunkJobs) AtIndex(index uint64) (module.Job, error) {
	locator, err := j.locators.AtIndex(index)
	if err != nil {
		return nil, fmt.Errorf("could not read chunk: %w", err)
	}
	return ChunkLocatorToJob(locator), nil
}

func (j *ChunkJobs) Head() (uint64, error) {
	return j.locators.LatestIndex()
}

// Worker receives job from job consumer and converts it back to Chunk
// for engine to process
type Worker struct {
	engine   fetcher.AssignedChunkProcessor
	consumer *ChunkConsumer
}

func NewWorker(engine fetcher.AssignedChunkProcessor) *Worker {
	return &Worker{
		engine: engine,
	}
}

// Run converts the job to Chunk, it's guaranteed to work, because
// ChunkJobs converted chunk into job symmetrically
func (w *Worker) Run(job module.Job) error {
	chunk, err := JobToChunkLocator(job)
	if err != nil {
		return err
	}
	w.engine.ProcessAssignedChunk(chunk)

	return nil
}

func (w *Worker) Notify(chunkID flow.Identifier) {
	jobID := locatorIDToJobID(chunkID)
	w.consumer.NotifyJobIsDone(jobID)
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
	chunksQueue storage.ChunksQueue, // to read jobs (chunks) from
	engine fetcher.AssignedChunkProcessor, // to process jobs (chunks)
	maxProcessing int64, // max number of jobs to be processed in parallel
) *ChunkConsumer {
	worker := NewWorker(engine)
	engine.WithChunkConsumerNotifier(worker)

	jobs := &ChunkJobs{locators: chunksQueue}

	// TODO: adding meta to logger
	consumer := jobqueue.NewConsumer(
		log, jobs, processedIndex, worker, maxProcessing,
	)

	chunkConsumer := &ChunkConsumer{consumer}

	worker.consumer = chunkConsumer

	return chunkConsumer
}

func (c *ChunkConsumer) NotifyJobIsDone(jobID module.JobID) {
	c.consumer.NotifyJobIsDone(jobID)
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
