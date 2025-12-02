package factory

// CreateFetcherConfig holds configuration parameters for creating a Fetcher.
type CreateFetcherConfig struct {
	// MaxProcessing is the maximum number of jobs to process concurrently.
	MaxProcessing uint64
	// MaxSearchAhead is the maximum number of jobs beyond processedIndex to process. 0 means no limit.
	MaxSearchAhead uint64
}

const DefaultMaxProcessing = 10
const DefaultMaxSearchAhead = 20
