package factory

import "github.com/onflow/flow-go/engine/access/collection_sync"

// ProgressReader aggregates progress from multiple backends and returns the maximum
// processed height. It can be initialized with an optional lastProgress value and
// two optional backends: executionDataProcessor and collectionFetcher.
type ProgressReader struct {
	lastProgress           uint64
	executionDataProcessor collection_sync.ProgressReader
	collectionFetcher      collection_sync.ProgressReader
}

var _ collection_sync.ProgressReader = (*ProgressReader)(nil)

// NewProgressReader creates a new ProgressReader initialized with lastProgress.
// Backends can be added using SetExecutionDataProcessor and SetCollectionFetcher.
func NewProgressReader(lastProgress uint64) *ProgressReader {
	return &ProgressReader{
		lastProgress:           lastProgress,
		executionDataProcessor: nil,
		collectionFetcher:      nil,
	}
}

// SetExecutionDataProcessor sets the execution data processor backend.
func (pr *ProgressReader) SetExecutionDataProcessor(backend collection_sync.ProgressReader) {
	pr.executionDataProcessor = backend
}

// SetCollectionFetcher sets the collection fetcher backend.
func (pr *ProgressReader) SetCollectionFetcher(backend collection_sync.ProgressReader) {
	pr.collectionFetcher = backend
}

// ProcessedHeight returns the maximum processed height from the available backends.
// If both backends are available, it returns the maximum of their progress and lastProgress.
// If only one backend is available, it returns the maximum of that backend's progress and lastProgress.
// If neither backend is available, it returns lastProgress.
func (pr *ProgressReader) ProcessedHeight() uint64 {
	hasExecutionData := pr.executionDataProcessor != nil
	hasCollectionFetcher := pr.collectionFetcher != nil

	if hasExecutionData && hasCollectionFetcher {
		execHeight := pr.executionDataProcessor.ProcessedHeight()
		collectionHeight := pr.collectionFetcher.ProcessedHeight()
		max := execHeight
		if collectionHeight > max {
			max = collectionHeight
		}
		if pr.lastProgress > max {
			max = pr.lastProgress
		}
		return max
	}

	if hasExecutionData {
		execHeight := pr.executionDataProcessor.ProcessedHeight()
		if pr.lastProgress > execHeight {
			return pr.lastProgress
		}
		return execHeight
	}

	if hasCollectionFetcher {
		collectionHeight := pr.collectionFetcher.ProcessedHeight()
		if pr.lastProgress > collectionHeight {
			return pr.lastProgress
		}
		return collectionHeight
	}

	return pr.lastProgress
}
