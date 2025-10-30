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
	data    []flow.LightTransactionResult
	results storage.LightTransactionResults
	blockID flow.Identifier
}

func NewResultsStore(
	data []flow.LightTransactionResult,
	results storage.LightTransactionResults,
	blockID flow.Identifier,
) *ResultsStore {
	return &ResultsStore{
		data:    data,
		results: results,
		blockID: blockID,
	}
}

// Persist saves and indexes all transaction results (light representation) for the block as part of the
// provided database batch. The caller must acquire [storage.LockInsertLightTransactionResult] and hold
// it until the write batch has been committed.
// Will return an error if the transaction results for the block already exist.
//
// No error returns are expected during normal operations
func (r *ResultsStore) Persist(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
	err := r.results.BatchStore(lctx, rw, r.blockID, r.data)
	if err != nil {
		return fmt.Errorf("could not add transaction results to batch: %w", err)
	}
	return nil
}
