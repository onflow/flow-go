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
	genericReceipts storage.ExecutionReceipts
	db              storage.DB
	cache           *Cache[flow.Identifier, *flow.ExecutionReceipt]
	// preventing dirty reads when checking if a different my receipt has been
	// indexed for the same block
	indexingMyReceipt *sync.Mutex
}

// NewMyExecutionReceipts creates instance of MyExecutionReceipts which is a wrapper wrapper around badger.ExecutionReceipts
// It's useful for execution nodes to keep track of produced execution receipts.
func NewMyExecutionReceipts(collector module.CacheMetrics, db storage.DB, receipts storage.ExecutionReceipts) *MyExecutionReceipts {
	indexingMyReceipt := new(sync.Mutex)

	store := func(rw storage.ReaderBatchWriter, blockID flow.Identifier, receipt *flow.ExecutionReceipt) error {
		// the lock guarantees that no other thread can concurrently update the index.
		// Note, we should not unlock the lock after this function returns, because the data is not yet persisted, the result
		// of whether there was a different own receipt for the same block might be stale, therefore, we should not unlock
		// the lock until the batch is committed.

		// the lock would not cause any deadlock, if
		// 1) there is no other lock in the batch operation.
		// 2) or there is other lock in the batch operation, but the locks are acquired and released in the same order.
		rw.Lock(indexingMyReceipt)

		// assemble DB operations to store receipt (no execution)
		err := receipts.BatchStore(receipt, rw)
		if err != nil {
			return err
		}

		// assemble DB operations to index receipt as one of my own (no execution)
		receiptID := receipt.ID()

		var savedReceiptID flow.Identifier
		err = operation.LookupOwnExecutionReceipt(rw.GlobalReader(), blockID, &savedReceiptID)
		if err == nil {
			if savedReceiptID == receiptID {
				// if we are storing same receipt we shouldn't error
				return nil
			}

			return fmt.Errorf("indexing my receipt %v failed: different receipt %v for the same block %v is already indexed", receiptID,
				savedReceiptID, blockID)
		}

		// exception
		if !errors.Is(err, storage.ErrNotFound) {
			return err
		}

		return operation.IndexOwnExecutionReceipt(rw.Writer(), blockID, receiptID)
	}

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

	return &MyExecutionReceipts{
		genericReceipts: receipts,
		db:              db,
		cache: newCache(collector, metrics.ResourceMyReceipt,
			withLimit[flow.Identifier, *flow.ExecutionReceipt](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
		indexingMyReceipt: indexingMyReceipt,
	}
}

// storeMyReceipt assembles the operations to retrieve my receipt for the given block ID.
func (m *MyExecutionReceipts) myReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	return m.cache.Get(m.db.Reader(), blockID) // assemble DB operations to retrieve receipt (no execution)
}

// BatchStoreMyReceipt stores blockID-to-my-receipt index entry keyed by blockID in a provided batch.
// No errors are expected during normal operation
// If entity fails marshalling, the error is wrapped in a generic error and returned.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
// If a different my receipt has been indexed for the same block, the error is wrapped in a generic error and returned.
func (m *MyExecutionReceipts) BatchStoreMyReceipt(receipt *flow.ExecutionReceipt, rw storage.ReaderBatchWriter) error {
	return m.cache.PutTx(rw, receipt.ExecutionResult.BlockID, receipt)
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
