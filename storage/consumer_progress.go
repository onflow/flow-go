package storage

// ConsumerProgress reads and writes the last processed index of the job in the job queue
type ConsumerProgress interface {
	// read the current processed index
	ProcessedIndex() (uint64, error)
	// update the processed index in the storage layer.
	// CAUTION: Not safe for concurrent use by multiple goroutines.
	// Use PersistentStrictMonotonicCounter instead to enforce strictly increasing values with concurrent writers.
	SetProcessedIndex(processed uint64) error
}

// ConsumerProgressFactory creates a new ConsumerProgress for the given consumer.
// This intermediary factory type exists to separate the concurrency-sensitive initialization from the rest of ConsumerProgress.
// The factory is intended to be used once in a non-concurrent environment, then discarded.
// The resulting ConsumerProgress instance is safe for concurrent use by multiple goroutines. 
type ConsumerProgressFactory interface {
	// InitConsumer initializes the consumer progress for the given consumer.
	// CAUTION: Must be called only once by one goroutine, per factory instance.
	InitConsumer(defaultIndex uint64) (ConsumerProgress, error)
}
