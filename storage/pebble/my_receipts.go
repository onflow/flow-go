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

// MyExecutionReceipts holds and indexes Execution Receipts.
// MyExecutionReceipts is implemented as a wrapper around badger.ExecutionReceipts
// The wrapper adds the ability to "MY execution receipt", from the viewpoint
// of an individual Execution Node.
type MyExecutionReceipts struct {
	genericReceipts *ExecutionReceipts
	db              *badger.DB
	cache           *Cache[flow.Identifier, *flow.ExecutionReceipt]
}

// NewMyExecutionReceipts creates instance of MyExecutionReceipts which is a wrapper wrapper around badger.ExecutionReceipts
// It's useful for execution nodes to keep track of produced execution receipts.
func NewMyExecutionReceipts(collector module.CacheMetrics, db *badger.DB, receipts *ExecutionReceipts) *MyExecutionReceipts {
	store := func(key flow.Identifier, receipt *flow.ExecutionReceipt) func(*transaction.Tx) error {
		// assemble DB operations to store receipt (no execution)
		storeReceiptOps := receipts.storeTx(receipt)
		// assemble DB operations to index receipt as one of my own (no execution)
		blockID := receipt.ExecutionResult.BlockID
		receiptID := receipt.ID()
		indexOwnReceiptOps := transaction.WithTx(func(tx *badger.Txn) error {
			err := operation.IndexOwnExecutionReceipt(blockID, receiptID)(tx)
			// check if we are storing same receipt
			if errors.Is(err, storage.ErrAlreadyExists) {
				var savedReceiptID flow.Identifier
				err := operation.LookupOwnExecutionReceipt(blockID, &savedReceiptID)(tx)
				if err != nil {
					return err
				}

				if savedReceiptID == receiptID {
					// if we are storing same receipt we shouldn't error
					return nil
				}

				return fmt.Errorf("indexing my receipt %v failed: different receipt %v for the same block %v is already indexed", receiptID,
					savedReceiptID, blockID)
			}
			return err
		})

		return func(tx *transaction.Tx) error {
			err := storeReceiptOps(tx) // execute operations to store receipt
			if err != nil {
				return fmt.Errorf("could not store receipt: %w", err)
			}
			err = indexOwnReceiptOps(tx) // execute operations to index receipt as one of my own
			if err != nil {
				return fmt.Errorf("could not index receipt as one of my own: %w", err)
			}
			return nil
		}
	}

	retrieve := func(blockID flow.Identifier) func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
		return func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
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

	return &MyExecutionReceipts{
		genericReceipts: receipts,
		db:              db,
		cache: newCache[flow.Identifier, *flow.ExecutionReceipt](collector, metrics.ResourceMyReceipt,
			withLimit[flow.Identifier, *flow.ExecutionReceipt](flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

// storeMyReceipt assembles the operations to store the receipt and marks it as mine (trusted).
func (m *MyExecutionReceipts) storeMyReceipt(receipt *flow.ExecutionReceipt) func(*transaction.Tx) error {
	return m.cache.PutTx(receipt.ExecutionResult.BlockID, receipt)
}

// storeMyReceipt assembles the operations to retrieve my receipt for the given block ID.
func (m *MyExecutionReceipts) myReceipt(blockID flow.Identifier) func(*badger.Txn) (*flow.ExecutionReceipt, error) {
	retrievalOps := m.cache.Get(blockID) // assemble DB operations to retrieve receipt (no execution)
	return func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
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
	return operation.RetryOnConflictTx(m.db, transaction.Update, m.storeMyReceipt(receipt))
}

// BatchStoreMyReceipt stores blockID-to-my-receipt index entry keyed by blockID in a provided batch.
// No errors are expected during normal operation
// If entity fails marshalling, the error is wrapped in a generic error and returned.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (m *MyExecutionReceipts) BatchStoreMyReceipt(receipt *flow.ExecutionReceipt, batch storage.BatchStorage) error {

	writeBatch := batch.GetWriter()

	err := m.genericReceipts.BatchStore(receipt, batch)
	if err != nil {
		return fmt.Errorf("cannot batch store generic execution receipt inside my execution receipt batch store: %w", err)
	}

	err = operation.BatchIndexOwnExecutionReceipt(receipt.ExecutionResult.BlockID, receipt.ID())(writeBatch)
	if err != nil {
		return fmt.Errorf("cannot batch index own execution receipt inside my execution receipt batch store: %w", err)
	}

	return nil
}

// MyReceipt retrieves my receipt for the given block.
// Returns storage.ErrNotFound if no receipt was persisted for the block.
func (m *MyExecutionReceipts) MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	tx := m.db.NewTransaction(false)
	defer tx.Discard()
	return m.myReceipt(blockID)(tx)
}

func (m *MyExecutionReceipts) RemoveIndexByBlockID(blockID flow.Identifier) error {
	return m.db.Update(operation.SkipNonExist(operation.RemoveOwnExecutionReceipt(blockID)))
}

// BatchRemoveIndexByBlockID removes blockID-to-my-execution-receipt index entry keyed by a blockID in a provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (m *MyExecutionReceipts) BatchRemoveIndexByBlockID(blockID flow.Identifier, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	return operation.BatchRemoveOwnExecutionReceipt(blockID)(writeBatch)
}
