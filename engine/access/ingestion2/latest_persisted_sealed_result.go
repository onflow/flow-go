package ingestion2

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// LatestPersistedSealedResult tracks the most recently persisted sealed execution result processed
// by the ingestion engine.
type LatestPersistedSealedResult struct {
	// resultID is the execution result ID of the most recently persisted sealed result.
	resultID flow.Identifier

	// height is the height of the most recently persisted sealed result's block.
	// This is the value stored in the consumer progress index.
	height uint64

	// cp is the consumer progress instance
	cp storage.ConsumerProgress

	// writeMu is used to prevent concurrent batch updates to the persisted height.
	// the critical section is fairly large, so use a separate mutex from the cached values.
	writeMu sync.Mutex

	// mu is used to protect access to resultID and height.
	mu sync.RWMutex
}

// NewLatestPersistedSealedResult creates a new LatestPersistedSealedResult instance.
// It initializes the consumer progress index using the provided initializer and initial height.
//
// No errors are expected during normal operation,
func NewLatestPersistedSealedResult(
	initialHeight uint64,
	initializer storage.ConsumerProgressInitializer,
	headers storage.Headers,
	results storage.ExecutionResults,
) (*LatestPersistedSealedResult, error) {
	// initialize the consumer progress, and set the initial height if this is the first run
	cp, err := initializer.Initialize(initialHeight)
	if err != nil {
		return nil, fmt.Errorf("could not initialize progress initializer: %w", err)
	}

	// get the actual height stored
	height, err := cp.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("could not get processed index: %w", err)
	}

	// finally, lookup the sealed resultID for the height
	header, err := headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header: %w", err)
	}

	result, err := results.ByBlockID(header.ID())
	if err != nil {
		return nil, fmt.Errorf("could not get result: %w", err)
	}

	return &LatestPersistedSealedResult{
		resultID: result.ID(),
		height:   height,
		cp:       cp,
	}, nil
}

// ResultID returns the ID of the latest persisted sealed result.
func (l *LatestPersistedSealedResult) ResultID() flow.Identifier {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.resultID
}

// Height returns the height of the latest persisted sealed result's block.
func (l *LatestPersistedSealedResult) Height() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.height
}

// BatchSet updates the latest persisted sealed result in a batch operation
// The resultID and height are added to the provided batch, and the local data is updated only after
// the batch is successfully committed.
//
// No errors are expected during normal operation,
func (l *LatestPersistedSealedResult) BatchSet(resultID flow.Identifier, height uint64, batch storage.ReaderBatchWriter) error {
	l.writeMu.Lock()

	batch.AddCallback(func(err error) {
		defer l.writeMu.Unlock()
		if err != nil {
			return
		}

		l.mu.Lock()
		defer l.mu.Unlock()

		l.resultID = resultID
		l.height = height
	})

	if err := l.cp.BatchSetProcessedIndex(height, batch); err != nil {
		return fmt.Errorf("could not add processed index update to batch: %w", err)
	}

	return nil
}
