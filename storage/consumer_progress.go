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
	// read the current processed index
	// any error returned are exceptions
	ProcessedIndex() (uint64, error)
	// update the processed index in the storage layer.
	// any error returned are exceptions
	SetProcessedIndex(processed uint64) error
}
