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

// ExecutionReceipts implements storage for execution receipts.
type ExecutionReceipts struct {
	db        *badger.DB
	results   *ExecutionResults
	cache     *Cache
	index     func(*badger.Txn, *flow.ExecutionReceipt) error
	byBlockId func(*badger.Txn, flow.Identifier) ([]*flow.ExecutionReceipt, error)
}

func newExecutionReceipts(collector module.CacheMetrics, db *badger.DB, results *ExecutionResults) *ExecutionReceipts {
	r := &ExecutionReceipts{
		db:      db,
		results: results,
	}
	store := func(key interface{}, val interface{}) func(tx *badger.Txn) error {
		return func(tx *badger.Txn) error {
			receipt := val.(*flow.ExecutionReceipt)
			// store the receipt-specific metadata
			err := operation.SkipDuplicates(operation.InsertExecutionReceiptMeta(receipt.ID(), receipt.Meta()))(tx)
			if err != nil {
				return fmt.Errorf("could not store receipt metadata: %w", err)
			}
			err = results.store(&receipt.ExecutionResult)(tx)
			if err != nil {
				return fmt.Errorf("could not store result: %w", err)
			}
			return r.index(tx, receipt)
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

	r.cache = newCache(collector,
		withLimit(flow.DefaultTransactionExpiry+100),
		withStore(store),
		withRetrieve(retrieve),
		withResource(metrics.ResourceReceipt))
	return r
}

func (r *ExecutionReceipts) store(receipt *flow.ExecutionReceipt) func(*badger.Txn) error {
	return r.cache.Put(receipt.ID(), receipt)
}

func (r *ExecutionReceipts) byID(receiptID flow.Identifier) func(*badger.Txn) (*flow.ExecutionReceipt, error) {
	return func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
		val, err := r.cache.Get(receiptID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*flow.ExecutionReceipt), nil
	}
}

func (r *ExecutionReceipts) Store(receipt *flow.ExecutionReceipt) error {
	return operation.RetryOnConflict(r.db.Update, r.store(receipt))
}

func (r *ExecutionReceipts) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byID(receiptID)(tx)
}

func (r *ExecutionReceipts) ByBlockID(blockID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byBlockId(tx, blockID)
}

func NewExecutionReceipts(collector module.CacheMetrics, db *badger.DB, results *ExecutionResults) *ExecutionReceipts {
	r := newExecutionReceipts(collector, db, results)
	r.index = func(tx *badger.Txn, receipt *flow.ExecutionReceipt) error {
		blockID := receipt.ExecutionResult.BlockID
		receiptID := receipt.ID()
		err := operation.IndexExecutionReceipt(blockID, receiptID)(tx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, storage.ErrAlreadyExists) {
			return err
		}

		// when trying to index a receipt for a block, and there is already a receipt indexed for this block,
		// double check if the indexed receipt has the same result.
		// if the result is the same, we could skip indexing the receipt
		// if the result is different, then return error
		var storedReceiptID flow.Identifier
		err = operation.LookupExecutionReceipt(blockID, &storedReceiptID)(tx)
		if err != nil {
			return fmt.Errorf("there is a receipt stored already, but cannot retrieve the ID of it: %w", err)
		}

		storedReceipt, err := r.byID(storedReceiptID)(tx)
		if err != nil {
			return fmt.Errorf("there is a receipt stored already, but cannot retrieve of it: %w", err)
		}

		storingReceipt, err := r.byID(receiptID)(tx)
		if err != nil {
			return fmt.Errorf("attempting to index a receipt, but the receipt is not stored yet: %w", err)
		}

		storedResultID := storedReceipt.ExecutionResult.ID()
		storingResultID := storingReceipt.ExecutionResult.ID()
		if storedResultID != storingResultID {
			return fmt.Errorf(
				"storing receipt that is different from the already stored one for block: %v, storing receipt: %v, stored receipt: %v. storing result: %v, stored result: %v, %w",
				blockID, receiptID, storedReceiptID, storingResultID, storedResultID, storage.ErrDataMismatch)
		}

		return nil
	}
	r.byBlockId = func(tx *badger.Txn, blockID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
		var receiptID flow.Identifier
		err := operation.LookupExecutionReceipt(blockID, &receiptID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup receipt ID: %w", err)
		}
		receipt, err := r.byID(receiptID)(tx)
		if err != nil {
			return nil, err
		}
		return []*flow.ExecutionReceipt{receipt}, nil
	}
	return r
}

func NewAllExecutionReceipts(collector module.CacheMetrics, db *badger.DB, results *ExecutionResults) *ExecutionReceipts {
	r := newExecutionReceipts(collector, db, results)
	r.index = func(tx *badger.Txn, receipt *flow.ExecutionReceipt) error {
		blockID := receipt.ExecutionResult.BlockID
		receiptID := receipt.ID()

		err := operation.IndexExecutionReceipts(blockID, receiptID)(tx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, storage.ErrAlreadyExists) {
			return err
		}

		return nil
	}
	r.byBlockId = func(tx *badger.Txn, blockID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
		var receipts []*flow.ExecutionReceipt
		var receiptIDs []flow.Identifier

		err := operation.LookupExecutionReceipts(blockID, &receiptIDs)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not find receipt index for block: %w", err)
		}

		for _, id := range receiptIDs {
			receipt, err := r.byID(id)(tx)
			if err != nil {
				return nil, fmt.Errorf("could not find receipt by id: %v, %w", id, err)
			}
			receipts = append(receipts, receipt)
		}

		return receipts, nil
	}
	return r
}
