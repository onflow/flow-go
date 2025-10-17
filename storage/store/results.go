package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ExecutionResults implements persistent storage for execution results.
type ExecutionResults struct {
	db         storage.DB
	cache      *Cache[flow.Identifier, *flow.ExecutionResult]
	indexCache *Cache[flow.Identifier, flow.Identifier]
}

var _ storage.ExecutionResults = (*ExecutionResults)(nil)

func NewExecutionResults(collector module.CacheMetrics, db storage.DB) *ExecutionResults {

	store := func(rw storage.ReaderBatchWriter, resultID flow.Identifier, result *flow.ExecutionResult) error {
		return operation.InsertExecutionResult(rw.Writer(), resultID, result)
	}

	retrieve := func(r storage.Reader, resultID flow.Identifier) (*flow.ExecutionResult, error) {
		var result flow.ExecutionResult
		err := operation.RetrieveExecutionResult(r, resultID, &result)
		return &result, err
	}

	retrieveByBlockID := func(r storage.Reader, blockID flow.Identifier) (flow.Identifier, error) {
		var resultID flow.Identifier
		err := operation.LookupExecutionResult(r, blockID, &resultID)
		return resultID, err
	}

	res := &ExecutionResults{
		db: db,
		cache: newCache(collector, metrics.ResourceResult,
			withLimit[flow.Identifier, *flow.ExecutionResult](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),

		indexCache: newCache(collector, metrics.ResourceResult,
			// this API is only used to fetch result for last executed block, so in happy case, it only need to be 1,
			// we use 100 here to be more resilient to forks
			withLimit[flow.Identifier, flow.Identifier](100),
			withStoreWithLock(operation.IndexOwnOrSealedExecutionResult),
			withRetrieve(retrieveByBlockID),
		),
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
	resultID, err := r.IDByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not lookup execution result ID: %w", err)
	}
	return r.byID(resultID)
}

// BatchIndex indexes an execution result by block ID in a given batch
// The caller must acquire [storage.LockIndexExecutionResult]
// It returns [storage.ErrDataMismatch] if there is already an indexed result for the given blockID,
// but it is different from the given resultID.
func (r *ExecutionResults) BatchIndex(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, resultID flow.Identifier) error {
	return r.indexCache.PutWithLockTx(lctx, rw, blockID, resultID)
}

// BatchStore stores an execution result in a given batch
// No error is expected during normal operation.
func (r *ExecutionResults) BatchStore(result *flow.ExecutionResult, batch storage.ReaderBatchWriter) error {
	return r.store(batch, result)
}

// ByID retrieves an execution result by its ID. Returns `ErrNotFound` if `resultID` is unknown.
func (r *ExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	return r.byID(resultID)
}

// ByBlockID retrieves an execution result by block ID.
// It returns [storage.ErrNotFound] if `blockID` refers to a block which is unknown, or for which a trusted (sealed or executed by this node) execution result does not exist.
func (r *ExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	return r.byBlockID(blockID)
}

// IDByBlockID retrieves an execution result ID by block ID.
// It returns [storage.ErrNotFound] if `blockID` does not refer to a block executed by this node
func (r *ExecutionResults) IDByBlockID(blockID flow.Identifier) (flow.Identifier, error) {
	return r.indexCache.Get(r.db.Reader(), blockID)
}

// BatchRemoveIndexByBlockID removes blockID-to-executionResultID index entries keyed by blockID in a provided batch.
// No errors are expected during normal operation, even if no entries are matched.
func (r *ExecutionResults) BatchRemoveIndexByBlockID(blockID flow.Identifier, batch storage.ReaderBatchWriter) error {
	return operation.RemoveExecutionResultIndex(batch.Writer(), blockID)
}
