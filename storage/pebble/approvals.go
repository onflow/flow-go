package pebble

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// ResultApprovals implements persistent storage for result approvals.
type ResultApprovals struct {
	db    *pebble.DB
	cache *Cache[flow.Identifier, *flow.ResultApproval]
}

func NewResultApprovals(collector module.CacheMetrics, db *pebble.DB) *ResultApprovals {

	store := func(key flow.Identifier, val *flow.ResultApproval) func(rw operation.PebbleReaderWriter) error {
		return operation.OnlyWrite(operation.InsertResultApproval(val))
	}

	retrieve := func(approvalID flow.Identifier) func(pebble.Reader) (*flow.ResultApproval, error) {
		var approval flow.ResultApproval
		return func(r pebble.Reader) (*flow.ResultApproval, error) {
			err := operation.RetrieveResultApproval(approvalID, &approval)(r)
			return &approval, err
		}
	}

	res := &ResultApprovals{
		db: db,
		cache: newCache(collector, metrics.ResourceResultApprovals,
			withLimit[flow.Identifier, *flow.ResultApproval](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}

	return res
}

func (r *ResultApprovals) store(approval *flow.ResultApproval) func(operation.PebbleReaderWriter) error {
	return r.cache.PutTx(approval.ID(), approval)
}

func (r *ResultApprovals) byID(approvalID flow.Identifier) func(pebble.Reader) (*flow.ResultApproval, error) {
	return func(tx pebble.Reader) (*flow.ResultApproval, error) {
		val, err := r.cache.Get(approvalID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func (r *ResultApprovals) byChunk(resultID flow.Identifier, chunkIndex uint64) func(pebble.Reader) (*flow.ResultApproval, error) {
	return func(tx pebble.Reader) (*flow.ResultApproval, error) {
		var approvalID flow.Identifier
		err := operation.LookupResultApproval(resultID, chunkIndex, &approvalID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup result approval ID: %w", err)
		}
		return r.byID(approvalID)(tx)
	}
}

func (r *ResultApprovals) index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) func(operation.PebbleReaderWriter) error {
	return func(rw operation.PebbleReaderWriter) error {
		// When trying to index an approval for a result, and there is already
		// an approval for the result, double check if the indexed approval is
		// the same.
		// We don't allow indexing multiple approvals per chunk because the
		// store is only used within Verification nodes, and it is impossible
		// for a Verification node to compute different approvals for the same
		// chunk.
		var storedApprovalID flow.Identifier
		err := operation.LookupResultApproval(resultID, chunkIndex, &storedApprovalID)(rw)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("there is an approval stored already, but cannot retrieve it: %w", err)
		}

		err = operation.IndexResultApproval(resultID, chunkIndex, approvalID)(rw)
		if err != nil {
			return fmt.Errorf("could not index result approval: %w", err)
		}

		if storedApprovalID != flow.ZeroID && storedApprovalID != approvalID {
			return fmt.Errorf("attempting to store conflicting approval (result: %v, chunk index: %d): storing: %v, stored: %v. %w",
				resultID, chunkIndex, approvalID, storedApprovalID, storage.ErrDataMismatch)
		}

		return nil
	}
}

// Store stores a ResultApproval
func (r *ResultApprovals) Store(approval *flow.ResultApproval) error {
	return r.store(approval)(r.db)
}

// Index indexes a ResultApproval by chunk (ResultID + chunk index).
// operation is idempotent (repeated calls with the same value are equivalent to
// just calling the method once; still the method succeeds on each call).
func (r *ResultApprovals) Index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	err := r.index(resultID, chunkIndex, approvalID)(r.db)
	if err != nil {
		return fmt.Errorf("could not index result approval: %w", err)
	}
	return nil
}

// ByID retrieves a ResultApproval by its ID
func (r *ResultApprovals) ByID(approvalID flow.Identifier) (*flow.ResultApproval, error) {
	return r.byID(approvalID)(r.db)
}

// ByChunk retrieves a ResultApproval by result ID and chunk index. The
// ResultApprovals store is only used within a verification node, where it is
// assumed that there is never more than one approval per chunk.
func (r *ResultApprovals) ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error) {
	return r.byChunk(resultID, chunkIndex)(r.db)
}
