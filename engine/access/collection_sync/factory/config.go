package factory

import "time"

// CreateFetcherConfig holds configuration parameters for creating a Fetcher.
type CreateFetcherConfig struct {
	// MaxProcessing is the maximum number of jobs to process concurrently.
	MaxProcessing uint64
	// MaxSearchAhead is the maximum number of jobs beyond processedIndex to process. 0 means no limit.
	MaxSearchAhead uint64
	// RetryInterval is the interval for retrying missing collections. If 0, uses DefaultRetryInterval.
	RetryInterval time.Duration
}

const DefaultMaxProcessing = 10
const DefaultMaxSearchAhead = 20
