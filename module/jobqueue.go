package module

type JobID string

type Job interface {
	// each job has a unique ID for deduplication
	ID() JobID
}
