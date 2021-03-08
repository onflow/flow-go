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

// MyExecutionReceipts holds and indexes Execution Receipts.
// MyExecutionReceipts is implemented as a wrapper around badger.ExecutionReceipts
// The wrapper adds the ability to "MY execution receipt", from the viewpoint
// of an individual Execution Node.
type MyExecutionReceipts struct {
	genericReceipts *ExecutionReceipts
	db              *badger.DB
	cache           *Cache
}

func NewMyExecutionReceipts(collector module.CacheMetrics, db *badger.DB, receipts *ExecutionReceipts) *MyExecutionReceipts {
	store := func(key interface{}, val interface{}) func(tx *badger.Txn) error {
		receipt := val.(*flow.ExecutionReceipt)
		// assemble DB operations to store receipt (no execution)
		storeReceiptOps := receipts.store(receipt)
		// assemble DB operations to index receipt as one of my own (no execution)
		blockID := receipt.ExecutionResult.BlockID
		receiptID := receipt.ID()
		indexOwnReceiptOps := func(tx *badger.Txn) error {
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
				} else {
					return fmt.Errorf("could not index receipt %v execution receipt %v already indexed", receiptID,
						savedReceiptID)
				}
			}
			return err
		}

		return func(tx *badger.Txn) error {
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

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		blockID := key.(flow.Identifier)

		return func(tx *badger.Txn) (interface{}, error) {
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
		cache: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceMyReceipt)),
	}
}

// storeMyReceipt assembles the operations to store the receipt and marks it as mine (trusted).
func (m *MyExecutionReceipts) storeMyReceipt(receipt *flow.ExecutionReceipt) func(*badger.Txn) error {
	return m.cache.Put(receipt.ID(), receipt)
}

// storeMyReceipt assembles the operations to retrieve my receipt for the given block ID.
func (m *MyExecutionReceipts) myReceipt(blockID flow.Identifier) func(*badger.Txn) (*flow.ExecutionReceipt, error) {
	retrievalOps := m.cache.Get(blockID) // assemble DB operations to retrieve receipt (no execution)
	return func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
		val, err := retrievalOps(tx) // execute operations to retrieve receipt
		if err != nil {
			return nil, err
		}
		return val.(*flow.ExecutionReceipt), nil
	}
}

// StoreMyReceipt stores the receipt and marks it as mine (trusted).
func (m *MyExecutionReceipts) StoreMyReceipt(receipt *flow.ExecutionReceipt) error {
	return operation.RetryOnConflict(m.db.Update, m.storeMyReceipt(receipt))
}

// MyReceipt retrieves my receipt for the given block.
// Returns badger.ErrKeyNotFound if no receipt was persisted for the block.
func (m *MyExecutionReceipts) MyReceipt(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	tx := m.db.NewTransaction(false)
	defer tx.Discard()
	return m.myReceipt(blockID)(tx)
}
