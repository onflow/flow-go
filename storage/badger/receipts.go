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
	db      *badger.DB
	results *ExecutionResults
	cache   *Cache
}

func NewExecutionReceipts(collector module.CacheMetrics, db *badger.DB, results *ExecutionResults) *ExecutionReceipts {
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
		cache: newCache(collector,
			withLimit(flow.DefaultTransactionExpiry+100),
			withStore(store),
			withRetrieve(retrieve),
			withResource(metrics.ResourceReceipt)),
	}
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

func (r *ExecutionReceipts) byBlockID(blockID flow.Identifier) func(*badger.Txn) (*flow.ExecutionReceipt, error) {
	return func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
		var receiptID flow.Identifier
		err := operation.LookupExecutionReceipt(blockID, &receiptID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not lookup receipt ID: %w", err)
		}
		return r.byID(receiptID)(tx)
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

func (r *ExecutionReceipts) Index(blockID, receiptID flow.Identifier) error {
	return operation.RetryOnConflict(r.db.Update, func(tx *badger.Txn) error {

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
	})
}

func (r *ExecutionReceipts) IndexByExecutor(receipt *flow.ExecutionReceipt) error {
	blockID := receipt.ExecutionResult.BlockID
	executorID := receipt.ExecutorID
	receiptID := receipt.ID()
	return operation.RetryOnConflict(r.db.Update, func(tx *badger.Txn) error {

		err := operation.IndexExecutionReceiptByBlockIDExecutionID(blockID, executorID, receiptID)(tx)
		if err == nil {
			return nil
		}

		if !errors.Is(err, storage.ErrAlreadyExists) {
			return err
		}

		// when trying to index a receipt for a block, and there is already a receipt indexed for this block,
		// double check if the indexed receipt is the same
		var storedReceiptID flow.Identifier
		err = operation.LookupExecutionReceiptByBlockIDExecutionID(blockID, executorID, &storedReceiptID)(tx)
		if err != nil {
			return fmt.Errorf("there is a receipt stored already, but cannot retrieve the ID of it: %w", err)
		}

		_, err = r.byID(storedReceiptID)(tx)
		if err != nil {
			return fmt.Errorf("there is a receipt stored already, but cannot retrieve it (stored receiptID: %v): %w", storedReceiptID, err)
		}

		if storedReceiptID != receiptID {
			return fmt.Errorf(
				"storing receipt that is different from the already stored one for block: %v, execution ID: %v, storing receipt: %v, stored receipt: %v, %w",
				blockID, executorID, receiptID, storedReceiptID, storage.ErrDataMismatch)
		}

		return nil
	})
}

func (r *ExecutionReceipts) ByBlockID(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byBlockID(blockID)(tx)
}

func (r *ExecutionReceipts) ByBlockIDAllExecutionReceipts(blockID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
	var resultReceipts []*flow.ExecutionReceipt
	err := r.db.View(func(btx *badger.Txn) error {
		var receiptIDs []flow.Identifier

		// lookup all the receipts for the given blockID received from different execution nodes
		err := operation.LookupExecutionReceiptByBlockIDAllExecutionIDs(blockID, &receiptIDs)(btx)
		if err != nil {
			return fmt.Errorf("could not find receipt index for block: %w", err)
		}

		// execution result ID to execution receipt map to keep track of receipts by their result id
		var identicalReceipts = make(map[flow.Identifier][]*flow.ExecutionReceipt)

		// highest number of matching receipts seen so far
		maxMatchedReceiptCnt := 0
		// execution result id key for the highest number of matching receipts in the identicalReceipts map
		var maxMatchedReceiptResultID flow.Identifier

		// find the largest list of receipts which have the same result ID
		for _, id := range receiptIDs {
			receipt, err := r.byID(id)(btx)
			if err != nil {
				return fmt.Errorf("could not find receipt by id: %v, %w", id, err)
			}

			resultID := receipt.ExecutionResult.ID()
			receipts, ok := identicalReceipts[resultID]

			if ok {
				receipts = append(receipts, receipt)
			} else {
				receipts = []*flow.ExecutionReceipt{receipt}
				identicalReceipts[resultID] = receipts
			}

			currentMatchedReceiptCnt := len(receipts)
			if currentMatchedReceiptCnt > maxMatchedReceiptCnt {
				maxMatchedReceiptCnt = currentMatchedReceiptCnt
				maxMatchedReceiptResultID = resultID
			}
		}

		// return the largest list of receipts
		resultReceipts = identicalReceipts[maxMatchedReceiptResultID]
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("could not get receipts for block: %v, %w", blockID, err)
	}
	return resultReceipts, nil
}
