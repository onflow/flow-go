package match

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
)

// ChunkJob converts a Chunk into a Job to be used by job queue
type ChunkJob struct {
	Chunk *flow.Chunk
}

// ID converts chunk id into job id, which guarantees uniqueness
func (j *ChunkJob) ID() module.JobID {
	return chunkIDToJobID(j.Chunk.ID())
}

func chunkIDToJobID(chunkID flow.Identifier) module.JobID {
	return module.JobID(fmt.Sprintf("%v", chunkID))
}

func chunkToJob(chunk *flow.Chunk) *ChunkJob {
	return &ChunkJob{Chunk: chunk}
}

func jobToChunk(job storage.Job) *flow.Chunk {
	chunkjob, _ := job.(*ChunkJob)
	return chunkjob.Chunk
}

// ChunksJob wraps the storage layer to provide an abstraction for
// consumers to read jobs
type ChunksJob struct {
	chunks storage.ChunksQueue
}

func (j ChunksJob) AtIndex(index int64) (storage.Job, error) {
	chunk, err := j.chunks.AtIndex(index)
	if err != nil {
		return nil, fmt.Errorf("could not read chunk: %w", err)
	}
	return chunkToJob(chunk), nil
}

// Worker receives job from job consumer and converts it back to Chunk
// for engine to process
type Worker struct {
	engine   *Engine
	consumer *ChunkConsumer
}

// Run converts the job to Chunk, it's guaranteed to work, because
// ChunksJob converted chunk into job symmetrically
func (w *Worker) Run(job storage.Job) {
	chunk := jobToChunk(job)
	w.engine.ProcessMyChunk(chunk)
}

func (w *Worker) FinishProcessing(chunkID flow.Identifier) {
	jobID := chunkIDToJobID(chunkID)
	w.consumer.FinishJob(jobID)
}

// finishProcessing is for the worker's underneath engine to report a chunk
// has been processed without knowing the job queue
// it's a callback so that the worker can convert the chunk id into a job
// id, and notify the consumer about a finished job with the
type finishProcessing interface {
	FinishProcessing(chunkID flow.Identifier)
}

// ChunkConsumer consumes the jobs from the job queue, and pass it to the
// Worker for processing.
// It wraps the generic job consumer in order to be used as a ReadyDoneAware
// on startup
type ChunkConsumer struct {
	module.JobConsumer
}

func NewChunkConsumer(
	log zerolog.Logger,
	processedIndex storage.ConsumerProgress, // to persist the processed index
	chunksQueue storage.ChunksQueue, // to read jobs (chunks) from
	engine *Engine, // to process jobs (chunks)
	maxProcessing int64, // max number of jobs to be processed in parallel
	maxFinished int64, // when the next unprocessed job is not finished,
	// the max number of finished subsequent jobs before stopping processing more jobs
) (*ChunkConsumer, jobqueue.Worker) {
	worker := &Worker{engine: engine}
	engine.withFinishProcessing(worker)

	jobs := &ChunksJob{chunks: chunksQueue}
	consumer := jobqueue.NewConsumer(
		log, jobs, processedIndex, worker, maxProcessing, maxFinished,
	)

	chunkConsumer := &ChunkConsumer{consumer}
	worker.consumer = chunkConsumer

	return chunkConsumer, worker
}

func (c *ChunkConsumer) Ready() <-chan struct{} {
	err := c.Start()
	if err != nil {
		panic(fmt.Errorf("could not start the chunk consumer for match engine: %w", err))
	}

	ready := make(chan struct{})
	close(ready)
	return ready
}

func (c *ChunkConsumer) Done() <-chan struct{} {
	c.Stop()

	ready := make(chan struct{})
	close(ready)
	return ready
}
