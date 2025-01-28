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

// ResultApprovals implements persistent storage for result approvals.
type ResultApprovals struct {
	db    *badger.DB
	cache *Cache[flow.Identifier, *flow.ResultApproval]
}

func NewResultApprovals(collector module.CacheMetrics, db *badger.DB) *ResultApprovals {

	store := func(key flow.Identifier, val *flow.ResultApproval) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertResultApproval(val)))
	}

	retrieve := func(approvalID flow.Identifier) func(tx *badger.Txn) (*flow.ResultApproval, error) {
		var approval flow.ResultApproval
		return func(tx *badger.Txn) (*flow.ResultApproval, error) {
			err := operation.RetrieveResultApproval(approvalID, &approval)(tx)
			return &approval, err
		}
	}

	res := &ResultApprovals{
		db: db,
		cache: newCache[flow.Identifier, *flow.ResultApproval](collector, metrics.ResourceResultApprovals,
			withLimit[flow.Identifier, *flow.ResultApproval](flow.DefaultTransactionExpiry+100),
			withStore[flow.Identifier, *flow.ResultApproval](store),
			withRetrieve[flow.Identifier, *flow.ResultApproval](retrieve)),
	}

	return res
}

func (r *ResultApprovals) store(approval *flow.ResultApproval) func(*transaction.Tx) error {
	return r.cache.PutTx(approval.ID(), approval)
}

func (r *ResultApprovals) byID(approvalID flow.Identifier) func(*badger.Txn) (*flow.ResultApproval, error) {
	return func(tx *badger.Txn) (*flow.ResultApproval, error) {
		val, err := r.cache.Get(approvalID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (r *ResultApprovals) byChunk(resultID flow.Identifier, chunkIndex uint64) func(*badger.Txn) (*flow.ResultApproval, error) {
	return func(tx *badger.Txn) (*flow.ResultApproval, error) {
		var approvalID flow.Identifier
		err := operation.LookupResultApproval(resultID, chunkIndex, &approvalID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup result approval ID: %w", err)
		}
		return r.byID(approvalID)(tx)
	}
}

func (r *ResultApprovals) index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := operation.IndexResultApproval(resultID, chunkIndex, approvalID)(tx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, storage.ErrAlreadyExists) {
			return err
		}

		// When trying to index an approval for a result, and there is already
		// an approval for the result, double check if the indexed approval is
		// the same.
		// We don't allow indexing multiple approvals per chunk because the
		// store is only used within Verification nodes, and it is impossible
		// for a Verification node to compute different approvals for the same
		// chunk.
		var storedApprovalID flow.Identifier
		err = operation.LookupResultApproval(resultID, chunkIndex, &storedApprovalID)(tx)
		if err != nil {
			return fmt.Errorf("there is an approval stored already, but cannot retrieve it: %w", err)
		}

		if storedApprovalID != approvalID {
			return fmt.Errorf("attempting to store conflicting approval (result: %v, chunk index: %d): storing: %v, stored: %v. %w",
				resultID, chunkIndex, approvalID, storedApprovalID, storage.ErrDataMismatch)
		}

		return nil
	}
}

// Store stores a ResultApproval
func (r *ResultApprovals) Store(approval *flow.ResultApproval) error {
	return operation.RetryOnConflictTx(r.db, transaction.Update, r.store(approval))
}

// Index indexes a ResultApproval by chunk (ResultID + chunk index).
// operation is idempotent (repeated calls with the same value are equivalent to
// just calling the method once; still the method succeeds on each call).
func (r *ResultApprovals) Index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	err := operation.RetryOnConflict(r.db.Update, r.index(resultID, chunkIndex, approvalID))
	if err != nil {
		return fmt.Errorf("could not index result approval: %w", err)
	}
	return nil
}

// ByID retrieves a ResultApproval by its ID
func (r *ResultApprovals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byID(approvalID)(tx)
}

// ByChunk retrieves a ResultApproval by result ID and chunk index. The
// ResultApprovals store is only used within a verification node, where it is
// assumed that there is never more than one approval per chunk.
func (r *ResultApprovals) ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byChunk(resultID, chunkIndex)(tx)
}
