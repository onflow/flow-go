package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
	"github.com/onflow/flow-go/storage/operation"
)

// ExecutionResults implements persistent storage for execution results.
type ExecutionResults struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.ExecutionResult]
}

var _ storage.ExecutionResults = (*ExecutionResults)(nil)

func NewExecutionResults(collector module.CacheMetrics, db storage.DB) *ExecutionResults {

	store := func(rw storage.ReaderBatchWriter, _ flow.Identifier, result *flow.ExecutionResult) error {
		return operation.InsertExecutionResult(rw.Writer(), result)
	}

	retrieve := func(r storage.Reader, resultID flow.Identifier) (*flow.ExecutionResult, error) {
		var result flow.ExecutionResult
		err := operation.RetrieveExecutionResult(r, resultID, &result)
		return &result, err
	}

	res := &ExecutionResults{
		db: db,
		cache: newCache(collector, metrics.ResourceResult,
			withLimit[flow.Identifier, *flow.ExecutionResult](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return res
}

func (r *ExecutionResults) store(rw storage.ReaderBatchWriter, result *flow.ExecutionResult) error {
	return r.cache.PutTx(rw, result.ID(), result)
}

func (r *ExecutionResults) byID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	val, err := r.cache.Get(r.db.Reader(), resultID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (r *ExecutionResults) byBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	var resultID flow.Identifier
	err := operation.LookupExecutionResult(r.db.Reader(), blockID, &resultID)
	if err != nil {
		return nil, fmt.Errorf("could not lookup execution result ID: %w", err)
	}
	return r.byID(resultID)
}

func (r *ExecutionResults) index(w storage.Writer, blockID, resultID flow.Identifier, force bool) error {
	if !force {
		// when not forcing the index, check if the result is already indexed
		exist, err := operation.ExistExecutionResult(r.db.Reader(), blockID)
		if err != nil {
			return fmt.Errorf("could not check if execution result exists: %w", err)
		}

		// if the result is already indexed, check if the stored result is the same
		if exist {
			var storedResultID flow.Identifier
			err = operation.LookupExecutionResult(r.db.Reader(), blockID, &storedResultID)
			if err != nil {
				return fmt.Errorf("could not lookup execution result ID: %w", err)
			}

			if storedResultID != resultID {
				return fmt.Errorf("storing result that is different from the already stored one for block: %v, storing result: %v, stored result: %v. %w",
					blockID, resultID, storedResultID, storage.ErrDataMismatch)
			}

			// if the result is the same, we don't need to index it again
			return nil
		}

		// if the result is not indexed, we can index it
	}

	err := operation.IndexExecutionResult(w, blockID, resultID)
	if err == nil {
		return nil
	}

	return nil
}

func (r *ExecutionResults) Store(result *flow.ExecutionResult) error {
	return r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return r.store(rw, result)
	})
}

func (r *ExecutionResults) BatchStore(result *flow.ExecutionResult, batch storage.ReaderBatchWriter) error {
	return r.store(batch, result)
}

func (r *ExecutionResults) BatchIndex(blockID flow.Identifier, resultID flow.Identifier, batch storage.ReaderBatchWriter) error {
	return operation.IndexExecutionResult(batch.Writer(), blockID, resultID)
}

func (r *ExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	return r.byID(resultID)
}

// TODO: deprecated, should be removed when protocol data is moved pebble
func (r *ExecutionResults) ByIDTx(resultID flow.Identifier) func(tx *transaction.Tx) (*flow.ExecutionResult, error) {
	return func(tx *transaction.Tx) (*flow.ExecutionResult, error) {
		return nil, fmt.Errorf("not implemented")
	}
}

// Index indexes an execution result by block ID.
// Note: this method call is not concurrent safe, because it checks if the different result is already indexed
// by the same blockID, and if it is, it returns an error.
// The caller needs to ensure that there is no concurrent call to this method with the same blockID.
func (r *ExecutionResults) Index(blockID flow.Identifier, resultID flow.Identifier) error {
	err := r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return r.index(rw.Writer(), blockID, resultID, false)
	})

	if err != nil {
		return fmt.Errorf("could not index execution result: %w", err)
	}
	return nil
}

func (r *ExecutionResults) ForceIndex(blockID flow.Identifier, resultID flow.Identifier) error {
	err := r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return r.index(rw.Writer(), blockID, resultID, true)
	})

	if err != nil {
		return fmt.Errorf("could not index execution result: %w", err)
	}
	return nil
}

func (r *ExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	return r.byBlockID(blockID)
}

func (r *ExecutionResults) RemoveIndexByBlockID(blockID flow.Identifier) error {
	return r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveExecutionResultIndex(rw.Writer(), blockID)
	})
}

// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (r *ExecutionResults) BatchRemoveIndexByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) error {
	return operation.RemoveExecutionResultIndex(batch.Writer(), blockID)
}
