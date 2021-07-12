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
	DefaultJobIndex     = uint64(0)
	DefaultChunkWorkers = uint64(5)
)

// ChunkConsumer consumes the jobs from the job queue, and pass it to the
// Worker for processing.
// It wraps the generic job consumer in order to be used as a ReadyDoneAware
// on startup
type ChunkConsumer struct {
	consumer       module.JobConsumer
	chunkProcessor fetcher.AssignedChunkProcessor
	metrics        module.VerificationMetrics
}

func NewChunkConsumer(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	processedIndex storage.ConsumerProgress, // to persist the processed index
	chunksQueue storage.ChunksQueue, // to read jobs (chunks) from
	chunkProcessor fetcher.AssignedChunkProcessor, // to process jobs (chunks)
	maxProcessing uint64, // max number of jobs to be processed in parallel
) *ChunkConsumer {
	worker := NewWorker(chunkProcessor)
	chunkProcessor.WithChunkConsumerNotifier(worker)

	jobs := &ChunkJobs{locators: chunksQueue}

	lg := log.With().Str("module", "chunk_consumer").Logger()
	consumer := jobqueue.NewConsumer(lg, jobs, processedIndex, worker, maxProcessing)

	chunkConsumer := &ChunkConsumer{
		consumer:       consumer,
		chunkProcessor: chunkProcessor,
		metrics:        metrics,
	}

	worker.consumer = chunkConsumer

	return chunkConsumer
}

func (c *ChunkConsumer) NotifyJobIsDone(jobID module.JobID) {
	processedIndex := c.consumer.NotifyJobIsDone(jobID)
	c.metrics.OnChunkConsumerJobDone(processedIndex)
}

// Size returns number of in-memory chunk jobs that chunk consumer is processing.
func (c *ChunkConsumer) Size() uint {
	return c.consumer.Size()
}

func (c ChunkConsumer) Check() {
	c.consumer.Check()
}

func (c *ChunkConsumer) Ready() <-chan struct{} {
	err := c.consumer.Start(DefaultJobIndex)
	if err != nil {
		panic(fmt.Errorf("could not start the chunk consumer for match engine: %w", err))
	}

	return c.chunkProcessor.Ready()
}

func (c *ChunkConsumer) Done() <-chan struct{} {
	c.consumer.Stop()

	return c.chunkProcessor.Done()
}
