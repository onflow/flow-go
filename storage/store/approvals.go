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

// ResultApprovals implements persistent storage for result approvals.
//
// CAUTION suitable only for _Verification Nodes_ for persisting their _own_ approvals!
//   - In general, the Flow protocol requires multiple approvals for the same chunk from different
//     verification nodes. In other words, there are multiple different approvals for the same chunk.
//   - Internally, ResultApprovals populates an index from Executed Chunk ➜ ResultApproval. This is
//     *only safe* for Verification Nodes when tracking their own approvals (for the same ExecutionResult,
//     a Verifier will always produce the same approval)
type ResultApprovals struct {
	db          storage.DB
	cache       *Cache[flow.Identifier, *flow.ResultApproval]
	lockManager lockctx.Manager
}

var _ storage.ResultApprovals = (*ResultApprovals)(nil)

func NewResultApprovals(collector module.CacheMetrics, db storage.DB, lockManager lockctx.Manager) *ResultApprovals {
	retrieve := func(r storage.Reader, approvalID flow.Identifier) (*flow.ResultApproval, error) {
		var approval flow.ResultApproval
		err := operation.RetrieveResultApproval(r, approvalID, &approval)
		return &approval, err
	}

	return &ResultApprovals{
		lockManager: lockManager,
		db:          db,
		cache: newCache(collector, metrics.ResourceResultApprovals,
			withLimit[flow.Identifier, *flow.ResultApproval](flow.DefaultTransactionExpiry+100),
			withRetrieve(retrieve)),
	}
}

// StoreMyApproval returns a functor, whose execution
//   - will store the given ResultApproval
//   - and index it by result ID and chunk index.
//   - requires storage.LockIndexResultApproval lock to be held by the caller
//
// The functor's expected error returns during normal operation are:
//   - `storage.ErrDataMismatch` if a *different* approval for the same key pair (ExecutionResultID, chunk index) is already indexed
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
// It returns a functor so that some computation (such as computing approval ID) can be done
// before acquiring the lock.
func (r *ResultApprovals) StoreMyApproval(approval *flow.ResultApproval) func(lctx lockctx.Proof) error {
	// pre-compute the approval ID and encoded data to be stored
	// db operation is deferred until the returned function is called
	storing := operation.InsertAndIndexResultApproval(approval)

	return func(lctx lockctx.Proof) error {
		if !lctx.HoldsLock(storage.LockIndexResultApproval) {
			return fmt.Errorf("missing lock for index result approval")
		}

		return r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			storage.OnCommitSucceed(rw, func() {
				// the success callback is called after the lock is released, so
				// the id computation here would not increase the lock contention
				r.cache.Insert(approval.ID(), approval)
			})
			return storing(lctx, rw)
		})
	}
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
