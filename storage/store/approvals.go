package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ResultApprovals implements persistent storage for result approvals.
//
// CAUTION suitable only for _Verification Nodes_ for persisting their _own_ approvals!
//   - In general, the Flow protocol requires multiple approvals for the same chunk from different
//     verification nodes. In other words, there are multiple different approvals for the same chunk.
//   - Internally, ResultApprovals populates an index from Executed Chunk ➜ ResultApproval. This is
//     *only safe* for Verification Nodes when tracking their own approvals (for the same ExecutionResult,
//     a Verifier will always produce the same approval)
type ResultApprovals struct {
	db    storage.DB
	cache *Cache[flow.Identifier, *flow.ResultApproval]
}

var _ storage.ResultApprovals = (*ResultApprovals)(nil)

func NewResultApprovals(collector module.CacheMetrics, db storage.DB) *ResultApprovals {
	storeWithLock := func(lctx lockctx.Proof, rw storage.ReaderBatchWriter, key flow.Identifier, val *flow.ResultApproval) error {
		return operation.InsertResultApproval(lctx, rw.Writer(), val)
	}

	retrieve := func(r storage.Reader, approvalID flow.Identifier) (*flow.ResultApproval, error) {
		var approval flow.ResultApproval
		err := operation.RetrieveResultApproval(r, approvalID, &approval)
		return &approval, err
	}

	return &ResultApprovals{
		db: db,
		cache: newCache(collector, metrics.ResourceResultApprovals,
			withLimit[flow.Identifier, *flow.ResultApproval](flow.DefaultTransactionExpiry+100),
			withStoreWithLock(storeWithLock),
			withRetrieve(retrieve)),
	}
}

// Store stores my own ResultApproval
// No errors are expected during normal operations.
// it also indexes a ResultApproval by result ID and chunk index.
//
// CAUTION: the Flow protocol requires multiple approvals for the same chunk from different verification
// nodes. In other words, there are multiple different approvals for the same chunk. Therefore, the index
// Executed Chunk ➜ ResultApproval ID (populated here) is *only safe* to be used by Verification Nodes
// for tracking their own approvals.
//
// For the same ExecutionResult, a Verifier will always produce the same approval. Therefore, this operation
// is idempotent, i.e. repeated calls with the *same inputs* are equivalent to just calling the method once;
// still the method succeeds on each call. However, when attempting to index *different* ResultApproval IDs
// for the same key (resultID, chunkIndex) this method returns an exception, as this should never happen for
// a correct Verification Node indexing its own approvals.
func (r *ResultApprovals) StoreMyApproval(lctx lockctx.Proof, approval *flow.ResultApproval) error {
	if !lctx.HoldsLock(storage.LockMyResultApproval) {
		return fmt.Errorf("missing lock for index result approval")
	}

	approvalID := approval.ID()
	resultID := approval.Body.ExecutionResultID
	chunkIndex := approval.Body.ChunkIndex

	var storedApprovalID flow.Identifier
	err := operation.LookupResultApproval(r.db.Reader(), resultID, chunkIndex, &storedApprovalID)
	if err == nil {
		if storedApprovalID != approvalID {
			return fmt.Errorf("attempting to store conflicting approval (result: %v, chunk index: %d): storing: %v, stored: %v. %w",
				resultID, chunkIndex, approvalID, storedApprovalID, storage.ErrDataMismatch)
		}

		// already stored and indexed
		return nil
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not lookup result approval ID: %w", err)
	}

	// no approval found, store and index the approval

	return r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		// store approval
		err := r.cache.PutWithLockTx(lctx, rw, approvalID, approval)
		if err != nil {
			return err
		}

		// index approval
		return operation.IndexResultApproval(lctx, rw.Writer(), resultID, chunkIndex, approvalID)
	})
}

// ByID retrieves a ResultApproval by its ID
func (r *ResultApprovals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	val, err := r.cache.Get(r.db.Reader(), approvalID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// ByChunk retrieves a ResultApproval by result ID and chunk index. The
// ResultApprovals store is only used within a verification node, where it is
// assumed that there is never more than one approval per chunk.
func (r *ResultApprovals) ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	var approvalID flow.Identifier
	err := operation.LookupResultApproval(r.db.Reader(), resultID, chunkIndex, &approvalID)
	if err != nil {
		return nil, fmt.Errorf("could not lookup result approval ID: %w", err)
	}
	return r.ByID(approvalID)
}
