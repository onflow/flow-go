package store

import (
	"fmt"

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

	return &MyExecutionReceipts{
		genericReceipts: receipts,
		db:              db,
		cache: newCache(collector, metrics.ResourceMyReceipt,
			withLimit[flow.Identifier, *flow.ExecutionReceipt](flow.DefaultTransactionExpiry+100),
			withRetrieve(retrieve)),
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
