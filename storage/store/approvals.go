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
type ResultApprovals struct {
	db       storage.DB
	cache    *Cache[flow.Identifier, *flow.ResultApproval]
	indexing *sync.Mutex // preventing concurrent indexing of approvals
}

func NewResultApprovals(collector module.CacheMetrics, db storage.DB) *ResultApprovals {
	store := func(rw storage.ReaderBatchWriter, key flow.Identifier, val *flow.ResultApproval) error {
		return operation.InsertResultApproval(rw.Writer(), val)
	}

	retrieve := func(r storage.Reader, approvalID flow.Identifier) (*flow.ResultApproval, error) {
		var approval flow.ResultApproval
		err := operation.RetrieveResultApproval(r, approvalID, &approval)
		return &approval, err
	}

	res := &ResultApprovals{
		db: db,
		cache: newCache(collector, metrics.ResourceResultApprovals,
			withLimit[flow.Identifier, *flow.ResultApproval](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
		indexing: new(sync.Mutex),
	}

	return res
}

func (r *ResultApprovals) store(rw storage.ReaderBatchWriter, approval *flow.ResultApproval) error {
	return r.cache.PutTx(rw, approval.ID(), approval)
}

func (r *ResultApprovals) byID(reader storage.Reader, approvalID flow.Identifier) (*flow.ResultApproval, error) {
	val, err := r.cache.Get(reader, approvalID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (r *ResultApprovals) byChunk(reader storage.Reader, resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	var approvalID flow.Identifier
	err := operation.LookupResultApproval(reader, resultID, chunkIndex, &approvalID)
	if err != nil {
		return nil, fmt.Errorf("could not lookup result approval ID: %w", err)
	}
	return r.byID(reader, approvalID)
}

// CAUTION: Caller must acquire `indexing` lock.
func (r *ResultApprovals) index(rw storage.ReaderBatchWriter, resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
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
}

// Store stores a ResultApproval
func (r *ResultApprovals) Store(approval *flow.ResultApproval) error {
	return r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return r.store(rw, approval)
	})
}

// Index indexes a ResultApproval by chunk (ResultID + chunk index).
// operation is idempotent (repeated calls with the same value are equivalent to
// just calling the method once; still the method succeeds on each call).
func (r *ResultApprovals) Index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	// acquring the lock to prevent dirty reads when checking conflicted approvals
	// how it works:
	// the lock can only be acquired after the index operation is committed to the database,
	// since the index operation is the only operation that would affect the reads operation,
	// no writes can go through util the lock is released, so locking here could prevent dirty reads.
	r.indexing.Lock()
	defer r.indexing.Unlock()

	err := r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return r.index(rw, resultID, chunkIndex, approvalID)
	})

	if err != nil {
		return fmt.Errorf("could not index result approval: %w", err)
	}
	return nil
}

// ByID retrieves a ResultApproval by its ID
func (r *ResultApprovals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	return r.byID(r.db.Reader(), approvalID)
}

// ByChunk retrieves a ResultApproval by result ID and chunk index. The
// ResultApprovals store is only used within a verification node, where it is
// assumed that there is never more than one approval per chunk.
func (r *ResultApprovals) ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	return r.byChunk(r.db.Reader(), resultID, chunkIndex)
}
