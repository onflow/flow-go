package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// ExecutionResults implements persistent storage for execution results.
type ExecutionResults struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.ExecutionResult]
}

var _ storage.ExecutionResults = (*ExecutionResults)(nil)

func NewExecutionResults(collector module.CacheMetrics, db *pebble.DB) *ExecutionResults {

	store := func(_ flow.Identifier, result *flow.ExecutionResult) func(pebble.Writer) error {
		return operation.InsertExecutionResult(result)
	}

	retrieve := func(resultID flow.Identifier) func(tx pebble.Reader) (*flow.ExecutionResult, error) {
		return func(tx pebble.Reader) (*flow.ExecutionResult, error) {
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

func (r *ExecutionResults) store(result *flow.ExecutionResult) func(pebble.Writer) error {
	return r.cache.PutTx(result.ID(), result)
}

func (r *ExecutionResults) byID(resultID flow.Identifier) func(pebble.Reader) (*flow.ExecutionResult, error) {
	return func(tx pebble.Reader) (*flow.ExecutionResult, error) {
		val, err := r.cache.Get(resultID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (r *ExecutionResults) byBlockID(blockID flow.Identifier) func(pebble.Reader) (*flow.ExecutionResult, error) {
	return func(tx pebble.Reader) (*flow.ExecutionResult, error) {
		var resultID flow.Identifier
		err := operation.LookupExecutionResult(blockID, &resultID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup execution result ID: %w", err)
		}
		return r.byID(resultID)(tx)
	}
}

func (r *ExecutionResults) index(blockID, resultID flow.Identifier, force bool) func(pebble.Writer) error {
	return operation.IndexExecutionResult(blockID, resultID)
}

func (r *ExecutionResults) Store(result *flow.ExecutionResult) error {
	return r.store(result)(r.db)
}

func (r *ExecutionResults) BatchStore(result *flow.ExecutionResult, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	return operation.InsertExecutionResult(result)(operation.NewBatchWriter(writeBatch))
}

func (r *ExecutionResults) BatchIndex(blockID flow.Identifier, resultID flow.Identifier, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	return r.index(blockID, resultID, false)(operation.NewBatchWriter(writeBatch))
}

func (r *ExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	return r.byID(resultID)(r.db)
}

func (r *ExecutionResults) ByIDTx(resultID flow.Identifier) func(interface{}) (*flow.ExecutionResult, error) {
	return func(interface{}) (*flow.ExecutionResult, error) {
		return r.byID(resultID)(r.db)
	}
}

func (r *ExecutionResults) Index(blockID flow.Identifier, resultID flow.Identifier) error {
	return r.index(blockID, resultID, false)(r.db)
}

func (r *ExecutionResults) ForceIndex(blockID flow.Identifier, resultID flow.Identifier) error {
	return r.index(blockID, resultID, true)(r.db)
}

func (r *ExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	return r.byBlockID(blockID)(r.db)
}

func (r *ExecutionResults) RemoveIndexByBlockID(blockID flow.Identifier) error {
	return operation.RemoveExecutionResultIndex(blockID)(r.db)
}

// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
// No errors are expected during normal operation, even if no entries are matched.
// If pebble unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (r *ExecutionResults) BatchRemoveIndexByBlockID(blockID flow.Identifier, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	return operation.RemoveExecutionResultIndex(blockID)(operation.NewBatchWriter(writeBatch))
}
