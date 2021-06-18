package module

import (
	"github.com/onflow/flow-go/model/flow"
)

const (
	ConsumeProgressVerificationBlockHeight = "ConsumeProgressVerificationBlockHeight"
	ConsumeProgressVerificationChunkIndex  = "ConsumeProgressVerificationChunkIndex"
)

// JobID is a unique ID of the job.
type JobID string

type NewJobListener interface {
	// Check let the producer notify the consumer that a new job has been added, so that the consumer
	// can check if there is worker available to process that job.
	Check()
}

// JobConsumer consumes jobs from a job queue, and it remembers which job it has processed, and is able
// to resume processing from the next.
type JobConsumer interface {
	NewJobListener

	// Start starts processing jobs from a job queue. If this is the first time, a processed index
	// will be initialized in the storage. If it fails to initialize, an error will be returned
	Start(defaultIndex uint64) error

	// Stop gracefully stops the consumer from reading new jobs from the job queue. It does not stop
	// the existing worker finishing their jobs
	// It blocks until the existing worker finish processing the job
	Stop()

	// NotifyJobIsDone let the consumer know a job has been finished, so that consumer will take
	// the next job from the job queue if there are workers available. It returns the last processed job index.
	NotifyJobIsDone(JobID) uint64

	// Size returns the number of processing jobs in consumer.
	Size() uint
}

type Job interface {
	// each job has a unique ID for deduplication
	ID() JobID
}

// Jobs is the reader for an ordered job queue. Job can be fetched by the index,
// which start from 0
type Jobs interface {
	AtIndex(index uint64) (Job, error)

	// Head returns the index of the last job
	Head() (uint64, error)
}

type JobQueue interface {
	// Add a job to the job queue
	Add(job Job) error
}

// ProcessingNotifier is for the worker's underneath engine to report an entity
// has been processed without knowing the job queue.
// It is a callback so that the worker can convert the entity id into a job
// id, and notify the consumer about a finished job.
//
// At the current version, entities used in this interface are chunks and blocks ids.
type ProcessingNotifier interface {
	Notify(entityID flow.Identifier)
}
