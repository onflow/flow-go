package storage

import "github.com/onflow/flow-go/module"

type Job interface {
	// each job has a unique ID for deduplication
	ID() module.JobID
}

// Jobs is the reader for an ordered job queue. Job can be fetched by the index,
// which start from 0
type Jobs interface {
	AtIndex(index int64) (Job, error)
}

// ConsumerProgress reads and writes the last processed index of the job in the job queue
type ConsumerProgress interface {
	// read the current processed index
	ProcessedIndex() (int64, error)
	// insert the default processed index to the storage layer, can only be done once.
	// initialize for the second time will fail.
	InitProcessedIndex(defaultIndex int64) (bool, error)
	// update the processed index in the storage layer.
	// it will fail if InitProcessedIndex was never called.
	SetProcessedIndex(processed int64) error
}

type JobQueue interface {
	// Add a job to the job queue
	Add(job Job) error
}
