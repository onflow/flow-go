package store

import (
	"fmt"
	"sync"

	"github.com/jordanschalm/lockctx"

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

	// cacheMu is used to protect access to resultID and height.
	cacheMu sync.RWMutex
}

// NewLatestPersistedSealedResult creates a new LatestPersistedSealedResult instance.
//
// No errors are expected during normal operation,
func NewLatestPersistedSealedResult(
	progress storage.ConsumerProgress,
	headers storage.Headers,
	results storage.ExecutionResults,
) (*LatestPersistedSealedResult, error) {
	// load the height and resultID of the latest persisted sealed result
	height, err := progress.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("could not get processed index: %w", err)
	}

	header, err := headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header: %w", err)
	}

	// Note: the result-to-block relationship is indexed by the Access ingestion engine when a
	// result is sealed.
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

// BatchSet updates the latest persisted sealed result in a batch operation.
// The resultID and height are added to the provided batch, and the local data is updated only after
// the batch is successfully committed.
// The caller must hold [storage.LockUpdateLatestPersistedSealedResult].
//
// No errors are expected during normal operation.
func (l *LatestPersistedSealedResult) BatchSet(lctx lockctx.Proof, resultID flow.Identifier, height uint64, batch storage.ReaderBatchWriter) error {
	if !lctx.HoldsLock(storage.LockUpdateLatestPersistedSealedResult) {
		return fmt.Errorf("missing required lock: %s", storage.LockUpdateLatestPersistedSealedResult)
	}

	// cacheMu is used to protect access to the cached resultID and height values. This is an
	// optimization to avoid readers having to block during the batch operations, since they
	// can have arbitrarily long setup times.
	batch.AddCallback(func(err error) {
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
