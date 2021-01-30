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
	ProcessedIndex() (int64, error)
	InitProcessedIndex() (int64, error)
	SetProcessedIndex(processed int64) error
}

type JobQueue interface {
	Add(job Job) error
}
