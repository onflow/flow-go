package store

import (
	"errors"
	"fmt"
	"sync"

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
	db       storage.DB
	cache    *Cache[flow.Identifier, *flow.ResultApproval]
	indexing *sync.Mutex // preventing concurrent indexing of approvals
}

var _ storage.ResultApprovals = (*ResultApprovals)(nil)

func NewResultApprovals(collector module.CacheMetrics, db storage.DB) *ResultApprovals {
	store := func(rw storage.ReaderBatchWriter, key flow.Identifier, val *flow.ResultApproval) error {
		return operation.InsertResultApproval(rw.Writer(), val)
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
			withStore(store),
			withRetrieve(retrieve)),
		indexing: new(sync.Mutex),
	}
}

// Store stores a ResultApproval
func (r *ResultApprovals) Store(approval *flow.ResultApproval) error {
	return r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return r.cache.PutTx(rw, approval.ID(), approval)
	})
}

// Index indexes a ResultApproval by chunk (ResultID + chunk index).
// This operation is idempotent (repeated calls with the same value are equivalent to
// just calling the method once; still the method succeeds on each call).
//
// CAUTION: the Flow protocol requires multiple approvals for the same chunk from different verification
// nodes. In other words, there are multiple different approvals for the same chunk. Therefore, the index
// Executed Chunk ➜ ResultApproval ID (populated here) is *only safe* to be used by Verification Nodes
// for tracking their own approvals.
func (r *ResultApprovals) Index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	// For the same ExecutionResult, a correct Verifier will always produce the same approval. In other words,
	// if we have already indexed an approval for the pair (resultID, chunkIndex) we should never overwrite it
	// with a _different_ approval. We explicitly enforce that here to prevent state corruption.
	// The lock guarantees that no other thread can concurrently update the index. Thereby confirming that no value
	// is already stored for the given key (resultID, chunkIndex) and then updating the index (or aborting) is
	// synchronized into one atomic operation.
	r.indexing.Lock()
	defer r.indexing.Unlock()

	err := r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		var storedApprovalID flow.Identifier
		err := operation.LookupResultApproval(rw.GlobalReader(), resultID, chunkIndex, &storedApprovalID)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("could not lookup result approval ID: %w", err)
			}

			// no approval found, index the approval

			return operation.UnsafeIndexResultApproval(rw.Writer(), resultID, chunkIndex, approvalID)
		}

		// an approval is already indexed, double check if it is the same
		// We don't allow indexing multiple approvals per chunk because the
		// store is only used within Verification nodes, and it is impossible
		// for a Verification node to compute different approvals for the same
		// chunk.

		if storedApprovalID != approvalID {
			return fmt.Errorf("attempting to store conflicting approval (result: %v, chunk index: %d): storing: %v, stored: %v. %w",
				resultID, chunkIndex, approvalID, storedApprovalID, storage.ErrDataMismatch)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("could not index result approval: %w", err)
	}
	return nil
}

// ByID retrieves a ResultApproval by its ID
func (r *ResultApprovals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	reader, err := r.db.Reader()
	if err != nil {
		return nil, err
	}

	val, err := r.cache.Get(reader, approvalID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// ByChunk retrieves a ResultApproval by result ID and chunk index. The
// ResultApprovals store is only used within a verification node, where it is
// assumed that there is never more than one approval per chunk.
func (r *ResultApprovals) ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	reader, err := r.db.Reader()
	if err != nil {
		return nil, err
	}

	var approvalID flow.Identifier
	err = operation.LookupResultApproval(reader, resultID, chunkIndex, &approvalID)
	if err != nil {
		return nil, fmt.Errorf("could not lookup result approval ID: %w", err)
	}
	return r.ByID(approvalID)
}
