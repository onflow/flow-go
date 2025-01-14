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

// MyExecutionReceipts holds and indexes Execution Receipts.
// MyExecutionReceipts is implemented as a wrapper around badger.ExecutionReceipts
// The wrapper adds the ability to "MY execution receipt", from the viewpoint
// of an individual Execution Node.
type MyExecutionReceipts struct {
	genericReceipts     *ExecutionReceipts
	db                  storage.DB
	cache               *Cache[flow.Identifier, *flow.ExecutionReceipt]
	indexingOwnReceipts sync.Mutex // lock to ensure only one receipt is stored per block
}

// NewMyExecutionReceipts creates instance of MyExecutionReceipts which is a wrapper wrapper around badger.ExecutionReceipts
// It's useful for execution nodes to keep track of produced execution receipts.
func NewMyExecutionReceipts(collector module.CacheMetrics, db storage.DB, receipts *ExecutionReceipts) *MyExecutionReceipts {
	mr := &MyExecutionReceipts{
		genericReceipts: receipts,
		db:              db,
	}

	store := func(rw storage.ReaderBatchWriter, key flow.Identifier, receipt *flow.ExecutionReceipt) error {
		mr.indexingOwnReceipts.Lock()
		rw.AddCallback(func(error) {
			// not release the lock until the batch is committed.
			mr.indexingOwnReceipts.Unlock()
		})

		blockID := receipt.ExecutionResult.BlockID
		receiptID := receipt.ID()
		var myOwnReceiptExecutedBefore flow.Identifier
		err := operation.LookupOwnExecutionReceipt(rw.GlobalReader(), blockID, &myOwnReceiptExecutedBefore)
		if err == nil {
			// if the indexed receipt is the same as the one we are storing, then we can skip the index
			if myOwnReceiptExecutedBefore == receiptID {
				return nil
			}

			return fmt.Errorf("cannot store index own receipt because a different one already stored for block %s: %s",
				blockID, myOwnReceiptExecutedBefore)
		}

		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not check if stored a receipt for the same block before: %w", err)
		}

		err = receipts.storeTx(rw, receipt)
		if err != nil {
			return fmt.Errorf("could not store receipt: %w", err)
		}

		err = operation.IndexOwnExecutionReceipt(rw.Writer(), blockID, receiptID)
		if err != nil {
			return fmt.Errorf("could not index own receipt: %w", err)
		}

		return nil
	}

	retrieve := func(r storage.Reader, blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
		var receiptID flow.Identifier
		err := operation.LookupOwnExecutionReceipt(r, blockID, &receiptID)
		if err != nil {
			return nil, fmt.Errorf("could not lookup receipt ID: %w", err)
		}
		receipt, err := receipts.byID(receiptID)
		if err != nil {
			return nil, err
		}
		return receipt, nil
	}

	return &MyExecutionReceipts{
		genericReceipts: receipts,
		db:              db,
		cache: newCache(collector, metrics.ResourceMyReceipt,
			withLimit[flow.Identifier, *flow.ExecutionReceipt](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

// storeMyReceipt assembles the operations to store the receipt and marks it as mine (trusted).
func (m *MyExecutionReceipts) storeMyReceipt(rw storage.ReaderBatchWriter, receipt *flow.ExecutionReceipt) error {
	return m.cache.PutTx(rw, receipt.ExecutionResult.BlockID, receipt)
}

// storeMyReceipt assembles the operations to retrieve my receipt for the given block ID.
func (m *MyExecutionReceipts) myReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	return m.cache.Get(m.db.Reader(), blockID) // assemble DB operations to retrieve receipt (no execution)
}

// StoreMyReceipt stores the receipt and marks it as mine (trusted). My
// receipts are indexed by the block whose result they compute. Currently,
// we only support indexing a _single_ receipt per block. Attempting to
// store conflicting receipts for the same block will error.
func (m *MyExecutionReceipts) StoreMyReceipt(receipt *flow.ExecutionReceipt) error {
	return m.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return m.storeMyReceipt(rw, receipt)
	})
}

// BatchStoreMyReceipt stores blockID-to-my-receipt index entry keyed by blockID in a provided batch.
// No errors are expected during normal operation
// If entity fails marshalling, the error is wrapped in a generic error and returned.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (m *MyExecutionReceipts) BatchStoreMyReceipt(receipt *flow.ExecutionReceipt, rw storage.ReaderBatchWriter) error {

	err := m.genericReceipts.BatchStore(receipt, rw)
	if err != nil {
		return fmt.Errorf("cannot batch store generic execution receipt inside my execution receipt batch store: %w", err)
	}

	err = operation.IndexOwnExecutionReceipt(rw.Writer(), receipt.ExecutionResult.BlockID, receipt.ID())
	if err != nil {
		return fmt.Errorf("cannot batch index own execution receipt inside my execution receipt batch store: %w", err)
	}

	return nil
}

// MyReceipt retrieves my receipt for the given block.
// Returns storage.ErrNotFound if no receipt was persisted for the block.
func (m *MyExecutionReceipts) MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	return m.myReceipt(blockID)
}

func (m *MyExecutionReceipts) RemoveIndexByBlockID(blockID flow.Identifier) error {
	return m.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveOwnExecutionReceipt(rw.Writer(), blockID)
	})
}

// BatchRemoveIndexByBlockID removes blockID-to-my-execution-receipt index entry keyed by a blockID in a provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (m *MyExecutionReceipts) BatchRemoveIndexByBlockID(blockID flow.Identifier, rw storage.ReaderBatchWriter) error {
	return operation.RemoveOwnExecutionReceipt(rw.Writer(), blockID)
}
