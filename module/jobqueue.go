package module

// JobID is a unique ID of the job.
type JobID string

// JobConsumer consumes jobs from a job queue, and it remembers which job it has processed, and is able
// to resume processing from the next.
type JobConsumer interface {
	// Start starts processing jobs from a job queue. If this is the first time, a processed index
	// will be initialized in the storage. If it fails to initialize, an error will be returned
	Start() error

	// Stop gracefully stops the consumer from reading new jobs from the job queue. It does not stop
	// the existing worker finishing their jobs
	Stop()

	// FinishJob let the workers notify consumer that a job has been finished, so that the consumer
	// can check if there is new job could be read from storage and give to a worker for processing
	FinishJob(JobID)

	// Check let the producer notify the consumer that a new job has been added, so that the consumer
	// can job if there is worker available to process that job.
	Check()
}
