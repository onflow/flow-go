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

// ExecutionReceipts implements storage for execution receipts.
type ExecutionReceipts struct {
	db      *badger.DB
	results *ExecutionResults
	cache   *Cache
}

// NewExecutionReceipts Creates ExecutionReceipts instance which is a database of receipts which
// supports storing and indexing receipts by receipt ID and block ID.
func NewExecutionReceipts(collector module.CacheMetrics, db *badger.DB, results *ExecutionResults, cacheSize uint) *ExecutionReceipts {
	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		receipt := val.(*flow.ExecutionReceipt)
		receiptID := receipt.ID()

		// assemble DB operations to store result (no execution)
		storeResultOps := results.store(&receipt.ExecutionResult)
		// assemble DB operations to index receipt (no execution)
		storeReceiptOps := transaction.WithTx(operation.SkipDuplicates(operation.InsertExecutionReceiptMeta(receiptID, receipt.Meta())))
		// assemble DB operations to index receipt by the block it computes (no execution)
		indexReceiptOps := transaction.WithTx(operation.SkipDuplicates(
			operation.IndexExecutionReceipts(receipt.ExecutionResult.BlockID, receiptID),
		))

		return func(tx *transaction.Tx) error {
			err := storeResultOps(tx) // execute operations to store results
			if err != nil {
				return fmt.Errorf("could not store result: %w", err)
			}
			err = storeReceiptOps(tx) // execute operations to store receipt-specific meta-data
			if err != nil {
				return fmt.Errorf("could not store receipt metadata: %w", err)
			}
			err = indexReceiptOps(tx)
			if err != nil {
				return fmt.Errorf("could not index receipt by the block it computes: %w", err)
			}
			return nil
		}
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		receiptID := key.(flow.Identifier)
		return func(tx *badger.Txn) (interface{}, error) {
			var meta flow.ExecutionReceiptMeta
			err := operation.RetrieveExecutionReceiptMeta(receiptID, &meta)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve receipt meta: %w", err)
			}
			result, err := results.byID(meta.ResultID)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve result: %w", err)
			}
			return flow.ExecutionReceiptFromMeta(meta, *result), nil
		}
	}

	return &ExecutionReceipts{
		db:      db,
		results: results,
		cache: newCache(collector, metrics.ResourceReceipt,
			withLimit(cacheSize),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

// storeMyReceipt assembles the operations to store an arbitrary receipt.
func (r *ExecutionReceipts) storeTx(receipt *flow.ExecutionReceipt) func(*transaction.Tx) error {
	return r.cache.PutTx(receipt.ID(), receipt)
}

func (r *ExecutionReceipts) byID(receiptID flow.Identifier) func(*badger.Txn) (*flow.ExecutionReceipt, error) {
	retrievalOps := r.cache.Get(receiptID) // assemble DB operations to retrieve receipt (no execution)
	return func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
		val, err := retrievalOps(tx) // execute operations to retrieve receipt
		if err != nil {
			return nil, err
		}
		return val.(*flow.ExecutionReceipt), nil
	}
}

func (r *ExecutionReceipts) byBlockID(blockID flow.Identifier) func(*badger.Txn) ([]*flow.ExecutionReceipt, error) {
	return func(tx *badger.Txn) ([]*flow.ExecutionReceipt, error) {
		var receiptIDs []flow.Identifier
		err := operation.LookupExecutionReceipts(blockID, &receiptIDs)(tx)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("could not find receipt index for block: %w", err)
		}

		var receipts []*flow.ExecutionReceipt
		for _, id := range receiptIDs {
			receipt, err := r.byID(id)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not find receipt with id %v: %w", id, err)
			}
			receipts = append(receipts, receipt)
		}
		return receipts, nil
	}
}

func (r *ExecutionReceipts) Store(receipt *flow.ExecutionReceipt) error {
	return operation.RetryOnConflictTx(r.db, transaction.Update, r.storeTx(receipt))
}

func (r *ExecutionReceipts) BatchStore(receipt *flow.ExecutionReceipt, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()

	err := r.results.BatchStore(&receipt.ExecutionResult, batch)
	if err != nil {
		return fmt.Errorf("cannot batch store execution result inside execution receipt batch store: %w", err)
	}

	err = operation.BatchInsertExecutionReceiptMeta(receipt.ID(), receipt.Meta())(writeBatch)
	if err != nil {
		return fmt.Errorf("cannot batch store execution meta inside execution receipt batch store: %w", err)
	}

	err = operation.BatchIndexExecutionReceipts(receipt.ExecutionResult.BlockID, receipt.ID())(writeBatch)
	if err != nil {
		return fmt.Errorf("cannot batch index execution receipt inside execution receipt batch store: %w", err)
	}

	return nil
}

func (r *ExecutionReceipts) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byID(receiptID)(tx)
}

func (r *ExecutionReceipts) ByBlockID(blockID flow.Identifier) (flow.ExecutionReceiptList, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byBlockID(blockID)(tx)
}

func (r *ExecutionReceipts) RemoveByBlockID(blockID flow.Identifier) error {
	receipt, err := r.ByBlockID(blockID)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not find receipt: %w", err)
	}

	return r.db.Update(operation.RemoveExecutionReceipt(blockID, receipt))
}
