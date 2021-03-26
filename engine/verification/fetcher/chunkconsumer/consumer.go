package chunkconsumer

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/storage"
)

const (
	DefaultJobIndex = uint64(0)
)

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
