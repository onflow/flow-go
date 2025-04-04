package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ExecutionResults implements persistent storage for execution results.
type ExecutionResults struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, *flow.ExecutionResult]
}

var _ storage.ExecutionResults = (*ExecutionResults)(nil)

func NewExecutionResults(collector module.CacheMetrics, db *badger.DB) *ExecutionResults {

	store := func(_ flow.Identifier, result *flow.ExecutionResult) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertExecutionResult(result)))
	}

	retrieve := func(resultID flow.Identifier) func(tx *badger.Txn) (*flow.ExecutionResult, error) {
		return func(tx *badger.Txn) (*flow.ExecutionResult, error) {
			var result flow.ExecutionResult
			err := operation.RetrieveExecutionResult(resultID, &result)(tx)
			return &result, err
		}
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

func (r *ExecutionResults) store(result *flow.ExecutionResult) func(*transaction.Tx) error {
	return r.cache.PutTx(result.ID(), result)
}

func (r *ExecutionResults) byID(resultID flow.Identifier) func(*badger.Txn) (*flow.ExecutionResult, error) {
	return func(tx *badger.Txn) (*flow.ExecutionResult, error) {
		val, err := r.cache.Get(resultID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (r *ExecutionResults) byBlockID(blockID flow.Identifier) func(*badger.Txn) (*flow.ExecutionResult, error) {
	return func(tx *badger.Txn) (*flow.ExecutionResult, error) {
		var resultID flow.Identifier
		err := operation.LookupExecutionResult(blockID, &resultID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup execution result ID: %w", err)
		}
		return r.byID(resultID)(tx)
	}
}

func (r *ExecutionResults) index(blockID, resultID flow.Identifier, force bool) func(*transaction.Tx) error {
	return func(tx *transaction.Tx) error {
		err := transaction.WithTx(operation.IndexExecutionResult(blockID, resultID))(tx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, storage.ErrAlreadyExists) {
			return err
		}

		if force {
			return transaction.WithTx(operation.ReindexExecutionResult(blockID, resultID))(tx)
		}

		// when trying to index a result for a block, and there is already a result indexed for this block,
		// double check if the indexed result is the same
		var storedResultID flow.Identifier
		err = transaction.WithTx(operation.LookupExecutionResult(blockID, &storedResultID))(tx)
		if err != nil {
			return fmt.Errorf("there is a result stored already, but cannot retrieve it: %w", err)
		}

		if storedResultID != resultID {
			return fmt.Errorf("storing result that is different from the already stored one for block: %v, storing result: %v, stored result: %v. %w",
				blockID, resultID, storedResultID, storage.ErrDataMismatch)
		}

		return nil
	}
}

func (r *ExecutionResults) Store(result *flow.ExecutionResult) error {
	return operation.RetryOnConflictTx(r.db, transaction.Update, r.store(result))
}

func (r *ExecutionResults) BatchStore(result *flow.ExecutionResult, batch storage.ReaderBatchWriter) error {
	return fmt.Errorf("not implemented")
}

func (r *ExecutionResults) BatchIndex(blockID flow.Identifier, resultID flow.Identifier, batch storage.ReaderBatchWriter) error {
	return fmt.Errorf("not implemented")
}

func (r *ExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byID(resultID)(tx)
}

func (r *ExecutionResults) ByIDTx(resultID flow.Identifier) func(*transaction.Tx) (*flow.ExecutionResult, error) {
	return func(tx *transaction.Tx) (*flow.ExecutionResult, error) {
		result, err := r.byID(resultID)(tx.DBTxn)
		return result, err
	}
}

func (r *ExecutionResults) Index(blockID flow.Identifier, resultID flow.Identifier) error {
	err := operation.RetryOnConflictTx(r.db, transaction.Update, r.index(blockID, resultID, false))
	if err != nil {
		return fmt.Errorf("could not index execution result: %w", err)
	}
	return nil
}

func (r *ExecutionResults) ForceIndex(blockID flow.Identifier, resultID flow.Identifier) error {
	err := operation.RetryOnConflictTx(r.db, transaction.Update, r.index(blockID, resultID, true))
	if err != nil {
		return fmt.Errorf("could not index execution result: %w", err)
	}
	return nil
}

func (r *ExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byBlockID(blockID)(tx)
}

func (r *ExecutionResults) RemoveIndexByBlockID(blockID flow.Identifier) error {
	return r.db.Update(operation.SkipNonExist(operation.RemoveExecutionResultIndex(blockID)))
}

// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (r *ExecutionResults) BatchRemoveIndexByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) error {
	return fmt.Errorf("not implemented")
}
