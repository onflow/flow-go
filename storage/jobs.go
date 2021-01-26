package storage

type JobID string

type Job interface {
	// each job has a unique ID for deduplication
	ID() JobID
}

type Jobs interface {
	AtIndex(index int64) (Job, error)
}

type ConsumerProgress interface {
	ProcessedIndex() (int64, error)
	InitProcessedIndex() (int64, error)
	SetProcessedIndex(processed int64) error
}
