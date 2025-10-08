package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

var _ PersisterStore = (*ResultsStore)(nil)

// ResultsStore handles persisting transaction results
type ResultsStore struct {
	inMemoryResults  *unsynchronized.LightTransactionResults
	persistedResults storage.LightTransactionResults
	blockID          flow.Identifier
	lockManager      storage.LockManager
}

func NewResultsStore(
	inMemoryResults *unsynchronized.LightTransactionResults,
	persistedResults storage.LightTransactionResults,
	blockID flow.Identifier,
	lockManager storage.LockManager,
) *ResultsStore {
	return &ResultsStore{
		inMemoryResults:  inMemoryResults,
		persistedResults: persistedResults,
		blockID:          blockID,
		lockManager:      lockManager,
	}
}

// Persist adds results to the batch.
// No errors are expected during normal operations
func (r *ResultsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	results, err := r.inMemoryResults.ByBlockID(r.blockID)
	if err != nil {
		return fmt.Errorf("could not get results: %w", err)
	}

	if len(results) > 0 {
		// Use storage.WithLock to acquire the necessary lock and store the results
		err := storage.WithLock(r.lockManager, storage.LockInsertLightTransactionResult, func(lctx lockctx.Context) error {
			return r.persistedResults.BatchStore(lctx, batch, r.blockID, results)
		})
		if err != nil {
			return fmt.Errorf("could not add transaction results to batch: %w", err)
		}
	}

	return nil
}
