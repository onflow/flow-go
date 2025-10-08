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
}

func NewResultsStore(
	inMemoryResults *unsynchronized.LightTransactionResults,
	persistedResults storage.LightTransactionResults,
	blockID flow.Identifier,
) *ResultsStore {
	return &ResultsStore{
		inMemoryResults:  inMemoryResults,
		persistedResults: persistedResults,
		blockID:          blockID,
	}
}

// Persist adds results to the batch.
// No errors are expected during normal operations
func (r *ResultsStore) Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error {
	results, err := r.inMemoryResults.ByBlockID(r.blockID)
	if err != nil {
		return fmt.Errorf("could not get results: %w", err)
	}

	err = r.persistedResults.BatchStore(lctx, batch, r.blockID, results)
	if err != nil {
		return fmt.Errorf("could not add transaction results to batch: %w", err)
	}

	return nil
}
