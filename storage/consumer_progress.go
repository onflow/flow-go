package storage

// ConsumerProgress reads and writes the last processed index of the job in the job queue
type ConsumerProgress interface {
	// read the current processed index
	ProcessedIndex() (int64, error)
	// insert the default processed index to the storage layer, can only be done once.
	// initialize for the second time will return storage.ErrAlreadyExists
	InitProcessedIndex(defaultIndex int64) error
	// update the processed index in the storage layer.
	// it will fail if InitProcessedIndex was never called.
	SetProcessedIndex(processed int64) error
}
