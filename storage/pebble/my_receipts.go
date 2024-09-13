package pebble

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// MyExecutionReceipts holds and indexes Execution Receipts.
// MyExecutionReceipts is implemented as a wrapper around pebble.ExecutionReceipts
// The wrapper adds the ability to "MY execution receipt", from the viewpoint
// of an individual Execution Node.
type MyExecutionReceipts struct {
	genericReceipts     *ExecutionReceipts
	db                  *pebble.DB
	cache               *Cache[flow.Identifier, *flow.ExecutionReceipt]
	indexingOwnReceipts sync.Mutex // lock to ensure only one receipt is stored per block
}

// NewMyExecutionReceipts creates instance of MyExecutionReceipts which is a wrapper wrapper around pebble.ExecutionReceipts
// It's useful for execution nodes to keep track of produced execution receipts.
func NewMyExecutionReceipts(collector module.CacheMetrics, db *pebble.DB, receipts *ExecutionReceipts) *MyExecutionReceipts {

	mr := &MyExecutionReceipts{
		genericReceipts: receipts,
		db:              db,
	}

	store := func(key flow.Identifier, receipt *flow.ExecutionReceipt) func(storage.PebbleReaderBatchWriter) error {
		// assemble DB operations to store receipt (no execution)
		storeReceiptOps := receipts.storeTx(receipt)
		// assemble DB operations to index receipt as one of my own (no execution)
		blockID := receipt.ExecutionResult.BlockID
		receiptID := receipt.ID()

		// check if the block already has a receipt stored
		indexOwnReceiptOps := operation.IndexOwnExecutionReceipt(blockID, receiptID)

		return func(rw storage.PebbleReaderBatchWriter) error {

			err := storeReceiptOps(rw) // execute operations to store receipt
			if err != nil {
				return fmt.Errorf("could not store receipt: %w", err)
			}

			r, w := rw.ReaderWriter()

			// acquiring the lock is necessary to avoid dirty reads when calling LookupOwnExecutionReceipt
			mr.indexingOwnReceipts.Lock()
			rw.AddCallback(func() {
				// not release the lock until the batch is committed.
				mr.indexingOwnReceipts.Unlock()
			})

			var myOwnReceiptExecutedBefore flow.Identifier
			err = operation.LookupOwnExecutionReceipt(blockID, &myOwnReceiptExecutedBefore)(r)
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

			err = indexOwnReceiptOps(w) // execute operations to index receipt as one of my own
			if err != nil {
				return fmt.Errorf("could not index receipt as one of my own: %w", err)
			}
			return nil
		}
	}

	retrieve := func(blockID flow.Identifier) func(tx pebble.Reader) (*flow.ExecutionReceipt, error) {
		return func(tx pebble.Reader) (*flow.ExecutionReceipt, error) {
			var receiptID flow.Identifier
			err := operation.LookupOwnExecutionReceipt(blockID, &receiptID)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not lookup receipt ID: %w", err)
			}
			receipt, err := receipts.byID(receiptID)(tx)
			if err != nil {
				return nil, err
			}
			return receipt, nil
		}
	}

	mr.cache =
		newCache(collector, metrics.ResourceMyReceipt,
			withLimit[flow.Identifier, *flow.ExecutionReceipt](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve))

	return mr
}

// storeMyReceipt assembles the operations to store the receipt and marks it as mine (trusted).
func (m *MyExecutionReceipts) storeMyReceipt(receipt *flow.ExecutionReceipt) func(storage.PebbleReaderBatchWriter) error {
	return m.cache.PutPebble(receipt.ExecutionResult.BlockID, receipt)
}

// storeMyReceipt assembles the operations to retrieve my receipt for the given block ID.
func (m *MyExecutionReceipts) myReceipt(blockID flow.Identifier) func(pebble.Reader) (*flow.ExecutionReceipt, error) {
	retrievalOps := m.cache.Get(blockID) // assemble DB operations to retrieve receipt (no execution)
	return func(tx pebble.Reader) (*flow.ExecutionReceipt, error) {
		val, err := retrievalOps(tx) // execute operations to retrieve receipt
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

// StoreMyReceipt stores the receipt and marks it as mine (trusted). My
// receipts are indexed by the block whose result they compute. Currently,
// we only support indexing a _single_ receipt per block. Attempting to
// store conflicting receipts for the same block will error.
func (m *MyExecutionReceipts) StoreMyReceipt(receipt *flow.ExecutionReceipt) error {
	return operation.WithReaderBatchWriter(m.db, m.storeMyReceipt(receipt))
}

// BatchStoreMyReceipt stores blockID-to-my-receipt index entry keyed by blockID in a provided batch.
// No errors are expected during normal operation
// If entity fails marshalling, the error is wrapped in a generic error and returned.
// If pebble unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (m *MyExecutionReceipts) BatchStoreMyReceipt(receipt *flow.ExecutionReceipt, batch storage.BatchStorage) error {

	err := m.genericReceipts.BatchStore(receipt, batch)
	if err != nil {
		return fmt.Errorf("cannot batch store generic execution receipt inside my execution receipt batch store: %w", err)
	}

	writer := operation.NewBatchWriter(batch.GetWriter())

	err = operation.IndexOwnExecutionReceipt(receipt.ExecutionResult.BlockID, receipt.ID())(writer)
	if err != nil {
		return fmt.Errorf("cannot batch index own execution receipt inside my execution receipt batch store: %w", err)
	}

	return nil
}

// MyReceipt retrieves my receipt for the given block.
// Returns storage.ErrNotFound if no receipt was persisted for the block.
func (m *MyExecutionReceipts) MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	return m.myReceipt(blockID)(m.db)
}

func (m *MyExecutionReceipts) RemoveIndexByBlockID(blockID flow.Identifier) error {
	return operation.RemoveOwnExecutionReceipt(blockID)(m.db)
}

// BatchRemoveIndexByBlockID removes blockID-to-my-execution-receipt index entry keyed by a blockID in a provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If pebble unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (m *MyExecutionReceipts) BatchRemoveIndexByBlockID(blockID flow.Identifier, batch storage.BatchStorage) error {
	writer := operation.NewBatchWriter(batch.GetWriter())
	return operation.RemoveOwnExecutionReceipt(blockID)(writer)
}
