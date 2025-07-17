package storage

// ConsumerProgressInitializer is a helper to initialize the consumer progress index in storage
// It prevents the consumer from being used before initialization
type ConsumerProgressInitializer interface {
	// Initialize takes a default index and initializes the consumer progress index in storage
	// Initialize must be concurrent safe, meaning if called by different modules, should only
	// initialize once.
	Initialize(defaultIndex uint64) (ConsumerProgress, error)
}

// ConsumerProgress reads and writes the last processed index of the job in the job queue
// It must be created by the ConsumerProgressInitializer, so that it can guarantee
// the ProcessedIndex and SetProcessedIndex methods are safe to use.
type ConsumerProgress interface {
	// ProcessedIndex returns the processed index for the consumer
	// No errors are expected during normal operation
	ProcessedIndex() (uint64, error)

	// SetProcessedIndex updates the processed index for the consumer
	// The caller must use ConsumerProgressInitializer to initialize the progress index in storage
	// No errors are expected during normal operation
	SetProcessedIndex(processed uint64) error

	// BatchSetProcessedIndex updates the processed index for the consumer within in provided batch
	// The caller must use ConsumerProgressInitializer to initialize the progress index in storage
	// No errors are expected during normal operation
	BatchSetProcessedIndex(processed uint64, batch ReaderBatchWriter) error
}
