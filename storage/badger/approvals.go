package badger

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// ResultApprovals implements persistent storage for result approvals.
type ResultApprovals struct {
	db       *badger.DB
	cache    *CacheB[flow.Identifier, *flow.ResultApproval]
	indexing *sync.Mutex // preventing concurrent indexing of approvals
}

func NewResultApprovals(collector module.CacheMetrics, db *badger.DB) *ResultApprovals {

	store := func(key flow.Identifier, val *flow.ResultApproval) func(storage.BadgerReaderBatchWriter) error {
		return storage.OnlyBadgerWriter(operation.InsertResultApproval(val))
	}

	retrieve := func(approvalID flow.Identifier) func(tx storage.Reader) (*flow.ResultApproval, error) {
		var approval flow.ResultApproval
		return func(tx storage.Reader) (*flow.ResultApproval, error) {
			err := operation.RetrieveResultApproval(approvalID, &approval)(tx)
			return &approval, err
		}
	}

	res := &ResultApprovals{
		db: db,
		cache: newCacheB[flow.Identifier, *flow.ResultApproval](collector, metrics.ResourceResultApprovals,
			withLimitB[flow.Identifier, *flow.ResultApproval](flow.DefaultTransactionExpiry+100),
			withStoreB[flow.Identifier, *flow.ResultApproval](store),
			withRetrieveB[flow.Identifier, *flow.ResultApproval](retrieve)),
		indexing: new(sync.Mutex),
	}

	return res
}

func (r *ResultApprovals) store(approval *flow.ResultApproval) func(storage.BadgerReaderBatchWriter) error {
	return r.cache.PutTx(approval.ID(), approval)
}

func (r *ResultApprovals) byID(approvalID flow.Identifier) func(storage.Reader) (*flow.ResultApproval, error) {
	return func(tx storage.Reader) (*flow.ResultApproval, error) {
		val, err := r.cache.Get(approvalID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (r *ResultApprovals) byChunk(resultID flow.Identifier, chunkIndex uint64) func(storage.Reader) (*flow.ResultApproval, error) {
	return func(tx storage.Reader) (*flow.ResultApproval, error) {
		var approvalID flow.Identifier
		err := operation.LookupResultApproval(resultID, chunkIndex, &approvalID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup result approval ID: %w", err)
		}
		return r.byID(approvalID)(tx)
	}
}

func (r *ResultApprovals) index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) func(storage.BadgerReaderBatchWriter) error {
	return func(tx storage.BadgerReaderBatchWriter) error {
		r, w := tx.ReaderWriter()

		var storedApprovalID flow.Identifier
		err := operation.LookupResultApproval(resultID, chunkIndex, &storedApprovalID)(r)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("could not lookup result approval ID: %w", err)
			}

			// no approval found, index the approval

			return operation.IndexResultApproval(resultID, chunkIndex, approvalID)(w)
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
}

// Store stores a ResultApproval
func (r *ResultApprovals) Store(approval *flow.ResultApproval) error {
	return operation.WithBadgerReaderBatchWriter(r.db, r.store(approval))
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

	err := operation.WithBadgerReaderBatchWriter(r.db, r.index(resultID, chunkIndex, approvalID))
	if err != nil {
		return fmt.Errorf("could not index result approval: %w", err)
	}
	return nil
}

// ByID retrieves a ResultApproval by its ID
func (r *ResultApprovals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	return r.byID(approvalID)(operation.ToReader(r.db))
}

// ByChunk retrieves a ResultApproval by result ID and chunk index. The
// ResultApprovals store is only used within a verification node, where it is
// assumed that there is never more than one approval per chunk.
func (r *ResultApprovals) ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	return r.byChunk(resultID, chunkIndex)(operation.ToReader(r.db))
}
