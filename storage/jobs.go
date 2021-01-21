package storage

type JobID string

type Job interface {
	// each job has a unique ID for deduplication
	ID() JobID
}

type Jobs interface {
	AtIndex(index int) (Job, error)
}

type ConsumerProgress interface {
	ProcessedIndex() (int, error)
	SetProcessedIndex(processed int) error
}
