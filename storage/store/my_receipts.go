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

// MyExecutionReceipts holds and indexes Execution Receipts.
// MyExecutionReceipts is implemented as a wrapper around badger.ExecutionReceipts
// The wrapper adds the ability to "MY execution receipt", from the viewpoint
// of an individual Execution Node.
type MyExecutionReceipts struct {
	genericReceipts storage.ExecutionReceipts
	db              storage.DB
	cache           *Cache[flow.Identifier, *flow.ExecutionReceipt]
}

// NewMyExecutionReceipts creates instance of MyExecutionReceipts which is a wrapper wrapper around badger.ExecutionReceipts
// It's useful for execution nodes to keep track of produced execution receipts.
func NewMyExecutionReceipts(collector module.CacheMetrics, db storage.DB, receipts storage.ExecutionReceipts) *MyExecutionReceipts {
	retrieve := func(r storage.Reader, blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
		var receiptID flow.Identifier
		err := operation.LookupOwnExecutionReceipt(r, blockID, &receiptID)
		if err != nil {
			return nil, fmt.Errorf("could not lookup receipt ID: %w", err)
		}
		receipt, err := receipts.ByID(receiptID)
		if err != nil {
			return nil, err
		}
		return receipt, nil
	}

	remove := func(rw storage.ReaderBatchWriter, blockID flow.Identifier) error {
		return operation.RemoveOwnExecutionReceipt(rw.Writer(), blockID)
	}

	return &MyExecutionReceipts{
		genericReceipts: receipts,
		db:              db,
		cache: newCache(collector, metrics.ResourceMyReceipt,
			withLimit[flow.Identifier, *flow.ExecutionReceipt](flow.DefaultTransactionExpiry+100),
			withRetrieve(retrieve),
			withRemove[flow.Identifier, *flow.ExecutionReceipt](remove),
		),
	}
}

// storeMyReceipt assembles the operations to retrieve my receipt for the given block ID.
func (m *MyExecutionReceipts) myReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	return m.cache.Get(m.db.Reader(), blockID) // assemble DB operations to retrieve receipt (no execution)
}

// BatchStoreMyReceipt stores blockID-to-my-receipt index entry keyed by blockID in a provided batch.
//
// If entity fails marshalling, the error is wrapped in a generic error and returned.
// If database unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
// It requires [storage.LockInsertMyReceipt] to be held.
//
// Expected error returns during *normal* operations:
//   - `storage.ErrDataMismatch` if a *different* receipt has already been indexed for the same block
func (m *MyExecutionReceipts) BatchStoreMyReceipt(lctx lockctx.Proof, receipt *flow.ExecutionReceipt, rw storage.ReaderBatchWriter) error {
	receiptID := receipt.ID()
	blockID := receipt.ExecutionResult.BlockID

	// add DB operation to batch for storing receipt (execution deferred until batch is committed)
	err := m.genericReceipts.BatchStore(receipt, rw)
	if err != nil {
		return err
	}

	// require [storage.LockInsertMyReceipt] to be held
	err = operation.IndexMyExecutionReceipt(lctx, rw, blockID, receiptID)
	if err != nil {
		return err
	}

	// TODO: ideally, adding the receipt to the cache on success, should be done by the cache itself
	storage.OnCommitSucceed(rw, func() {
		m.cache.Insert(blockID, receipt)
	})
	return nil
}

// MyReceipt retrieves my receipt for the given block.
// Returns storage.ErrNotFound if no receipt was persisted for the block.
func (m *MyExecutionReceipts) MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	return m.myReceipt(blockID)
}

// BatchRemoveIndexByBlockID removes blockID-to-my-execution-receipt index entry keyed by a blockID in a provided batch
// No errors are expected during normal operation, even if no entries are matched.
func (m *MyExecutionReceipts) BatchRemoveIndexByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return m.cache.RemoveTx(rw, blockID)
}
