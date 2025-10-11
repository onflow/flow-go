package stores

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var _ PersisterStore = (*ResultsStore)(nil)

// ResultsStore handles persisting transaction results
type ResultsStore struct {
	data             []flow.LightTransactionResult
	persistedResults storage.LightTransactionResults
	blockID          flow.Identifier
}

func NewResultsStore(
	data []flow.LightTransactionResult,
	persistedResults storage.LightTransactionResults,
	blockID flow.Identifier,
) *ResultsStore {
	return &ResultsStore{
		data:             data,
		persistedResults: persistedResults,
		blockID:          blockID,
	}
}

// Persist saves and indexes all transaction results (light representation) for our block as part of the
// provided database batch. The caller must acquire [storage.LockInsertLightTransactionResult] and hold
// it until the write batch has been committed.
// No error returns are expected during normal operations
func (r *ResultsStore) Persist(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
	if err := r.persistedResults.BatchStore(lctx, rw, r.blockID, r.data); err != nil {
		return fmt.Errorf("could not add transaction results to batch: %w", err)
	}
	return nil
}
