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
)

// ResultApprovals implements persistent storage for result approvals.
type ResultApprovals struct {
	db    *badger.DB
	cache *Cache
}

func NewResultApprovals(collector module.CacheMetrics, db *badger.DB) *ResultApprovals {

	store := func(key interface{}, val interface{}) func(tx *badger.Txn) error {
		approval := val.(*flow.ResultApproval)
		return operation.SkipDuplicates(operation.InsertResultApproval(approval))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		approvalID := key.(flow.Identifier)
		var approval flow.ResultApproval
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveResultApproval(approvalID, &approval)(tx)
			return &approval, err
		}
	}

	res := &ResultApprovals{
		db: db,
		cache: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceResult)),
	}

	return res
}

func (r *ResultApprovals) store(approval *flow.ResultApproval) func(*badger.Txn) error {
	return r.cache.Put(approval.ID(), approval)
}

func (r *ResultApprovals) byID(approvalID flow.Identifier) func(*badger.Txn) (*flow.ResultApproval, error) {
	return func(tx *badger.Txn) (*flow.ResultApproval, error) {
		val, err := r.cache.Get(approvalID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.ResultApproval), nil
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

		// when trying to index an approval for a result, and there is already
		// an approval for the result, double check if the indexed approval is
		// the same
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

func (r *ResultApprovals) Store(approval *flow.ResultApproval) error {
	return operation.RetryOnConflict(r.db.Update, r.store(approval))
}

func (r *ResultApprovals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byID(approvalID)(tx)
}

func (r *ResultApprovals) Index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	err := operation.RetryOnConflict(r.db.Update, r.index(resultID, chunkIndex, approvalID))
	if err != nil {
		return fmt.Errorf("could not index result approval: %w", err)
	}
	return nil
}

func (r *ResultApprovals) ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byChunk(resultID, chunkIndex)(tx)
}
