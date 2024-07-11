package storage

// ConsumerProgress reads and writes the last processed index of the job in the job queue
type ConsumerProgress interface {
	// ProcessedIndex returns current processed index
	// Errors:
	// storage.ErrNotFound if the consumer has not been initialized
	// No errors are expected during normal operation
	ProcessedIndex() (uint64, error)

	// InitProcessedIndex inserts the default processed index to the storage layer
	// It should only be called once.
	// Errors:
	// storage.ErrAlreadyExists is the consumer has already been initialized
	// No other errors are expected during normal operation
	InitProcessedIndex(defaultIndex uint64) error

	// SetProcessedIndex updates the processed index in the storage layer.
	// It will return a generic error if InitProcessedIndex was never called.
	// No errors are expected during normal operation.
	SetProcessedIndex(processed uint64) error

	// Consumer returns the consumer's name
	Consumer() string
}
