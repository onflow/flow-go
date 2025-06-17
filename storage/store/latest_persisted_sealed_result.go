package store

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ storage.LatestPersistedSealedResult = (*LatestPersistedSealedResult)(nil)

// LatestPersistedSealedResult tracks the most recently persisted sealed execution result processed
// by the Access ingestion engine.
type LatestPersistedSealedResult struct {
	// resultID is the execution result ID of the most recently persisted sealed result.
	resultID flow.Identifier

	// height is the height of the most recently persisted sealed result's block.
	// This is the value stored in the consumer progress index.
	height uint64

	// progress is the consumer progress instance
	progress storage.ConsumerProgress

	// batchMu is used to prevent concurrent batch updates to the persisted height.
	// the critical section is fairly large, so use a separate mutex from the cached values.
	batchMu sync.Mutex

	// cacheMu is used to protect access to resultID and height.
	cacheMu sync.RWMutex
}

// NewLatestPersistedSealedResult creates a new LatestPersistedSealedResult instance.
// It initializes the consumer progress index using the provided initializer and initial height.
// initialHeight must be for a sealed block.
//
// No errors are expected during normal operation,
func NewLatestPersistedSealedResult(
	initialHeight uint64,
	initializer storage.ConsumerProgressInitializer,
	headers storage.Headers,
	results storage.ExecutionResults,
) (*LatestPersistedSealedResult, error) {
	// initialize the consumer progress, and set the initial height if this is the first run
	progress, err := initializer.Initialize(initialHeight)
	if err != nil {
		return nil, fmt.Errorf("could not initialize progress initializer: %w", err)
	}

	// get the actual height stored
	height, err := progress.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("could not get processed index: %w", err)
	}

	// finally, lookup the sealed resultID for the height
	header, err := headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header: %w", err)
	}

	// the result/block relationship is indexed by the Access ingestion engine when a result is sealed.
	result, err := results.ByBlockID(header.ID())
	if err != nil {
		return nil, fmt.Errorf("could not get result: %w", err)
	}

	return &LatestPersistedSealedResult{
		resultID: result.ID(),
		height:   height,
		progress: progress,
	}, nil
}

// Latest returns the ID and height of the latest persisted sealed result.
func (l *LatestPersistedSealedResult) Latest() (flow.Identifier, uint64) {
	l.cacheMu.RLock()
	defer l.cacheMu.RUnlock()
	return l.resultID, l.height
}

// BatchSet updates the latest persisted sealed result in a batch operation
// The resultID and height are added to the provided batch, and the local data is updated only after
// the batch is successfully committed.
//
// No errors are expected during normal operation,
func (l *LatestPersistedSealedResult) BatchSet(resultID flow.Identifier, height uint64, batch storage.ReaderBatchWriter) error {
	// there are 2 mutexes used here:
	// - batchMu is used to prevent concurrent batch updates to the persisted height. Since this
	//   is a global variable, we need to ensure that only a single batch is in progress at a time.
	// - cacheMu is used to protect access to the cached resultID and height values. This is an
	//   optimization to avoid readers having to block during the batch operations, since they
	//   can have arbitrarily long setup times.
	l.batchMu.Lock()

	batch.AddCallback(func(err error) {
		defer l.batchMu.Unlock()
		if err != nil {
			return
		}

		l.cacheMu.Lock()
		defer l.cacheMu.Unlock()

		l.resultID = resultID
		l.height = height
	})

	if err := l.progress.BatchSetProcessedIndex(height, batch); err != nil {
		return fmt.Errorf("could not add processed index update to batch: %w", err)
	}

	return nil
}
