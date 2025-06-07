package ingestion2

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type LatestPersistedSealedResult struct {
	resultID flow.Identifier
	height   uint64

	cp storage.ConsumerProgress

	// writeMu is used to prevent concurrent writes to the resultID and height.
	// the critical section is fairly large, so use a separate mutex from the main one used for reads
	writeMu sync.Mutex

	// mu is used to protect access to resultID and height.
	mu sync.RWMutex
}

// NewLatestPersistedSealedResult creates a new LatestPersistedSealedResult instance.
// It initializes the consumer progress index using the provided initializer and initial height.
//
// No errors are expected during normal operation,
func NewLatestPersistedSealedResult(resultID flow.Identifier, height uint64, initializer storage.ConsumerProgressInitializer) (*LatestPersistedSealedResult, error) {
	cp, err := initializer.Initialize(height)
	if err != nil {
		return nil, fmt.Errorf("could not initialize progress initializer: %w", err)
	}

	return &LatestPersistedSealedResult{
		resultID: resultID,
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
		return fmt.Errorf("could not set processed index: %w", err)
	}

	return nil
}
