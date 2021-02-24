package module

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
	Start(defaultIndex int64) error

	// Stop gracefully stops the consumer from reading new jobs from the job queue. It does not stop
	// the existing worker finishing their jobs
	Stop()

	// NotifyJobIsDone let the workers notify consumer that a job has been finished, so that the consumer
	// can check if there is new job could be read from storage and give to a worker for processing
	NotifyJobIsDone(JobID)
}

type Job interface {
	// each job has a unique ID for deduplication
	ID() JobID
}

// Jobs is the reader for an ordered job queue. Job can be fetched by the index,
// which start from 0
type Jobs interface {
	AtIndex(index int64) (Job, error)
}

type JobQueue interface {
	// Add a job to the job queue
	Add(job Job) error
}
