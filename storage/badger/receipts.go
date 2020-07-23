package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// ExecutionReceipts implements storage for execution receipts.
type ExecutionReceipts struct {
	db      *badger.DB
	results *ExecutionResults
}

func NewExecutionReceipts(db *badger.DB, results *ExecutionResults) *ExecutionReceipts {
	return &ExecutionReceipts{
		db:      db,
		results: results,
	}
}

func (r *ExecutionReceipts) store(receipt *flow.ExecutionReceipt) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		// store the receipt-specific metadata
		err := operation.InsertExecutionReceiptMeta(receipt.ID(), receipt.Meta())(tx)
		if err != nil {
			return fmt.Errorf("could not store receipt metadata: %w", err)
		}
		// store the result -- don't error if the result has already been stored
		err = operation.SkipDuplicates(r.results.store(&receipt.ExecutionResult))(tx)
		if err != nil {
			return fmt.Errorf("could not store result: %w", err)
		}
		return nil
	}
}

func (r *ExecutionReceipts) byID(receiptID flow.Identifier) func(*badger.Txn) (*flow.ExecutionReceipt, error) {
	return func(tx *badger.Txn) (*flow.ExecutionReceipt, error) {
		var meta flow.ExecutionReceiptMeta
		err := operation.RetrieveExecutionReceiptMeta(receiptID, &meta)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve receipt meta: %w", err)
		}
		result, err := r.results.byID(meta.ResultID)(tx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve result: %w", err)
		}
		return flow.ExecutionReceiptFromMeta(meta, *result), nil
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
	return operation.RetryOnConflict(r.db.Update, operation.IndexExecutionReceipt(blockID, receiptID))
}

func (r *ExecutionReceipts) ByBlockID(blockID flow.Identifier) (*flow.ExecutionReceipt, error) {
	tx := r.db.NewTransaction(false)
	defer tx.Discard()
	return r.byBlockID(blockID)(tx)
}
