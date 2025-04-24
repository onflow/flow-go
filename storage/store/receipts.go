package store

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ExecutionReceipts implements storage for execution receipts.
type ExecutionReceipts struct {
	db      storage.DB
	results storage.ExecutionResults
	cache   *Cache[flow.Identifier, *flow.ExecutionReceipt]
}

// NewExecutionReceipts Creates ExecutionReceipts instance which is a database of receipts which
// supports storing and indexing receipts by receipt ID and block ID.
func NewExecutionReceipts(collector module.CacheMetrics, db storage.DB, results storage.ExecutionResults, cacheSize uint) *ExecutionReceipts {
	store := func(rw storage.ReaderBatchWriter, receiptTD flow.Identifier, receipt *flow.ExecutionReceipt) error {
		receiptID := receipt.ID()

		err := results.BatchStore(&receipt.ExecutionResult, rw)
		if err != nil {
			return fmt.Errorf("could not store result: %w", err)
		}
		err = operation.InsertExecutionReceiptStub(rw.Writer(), receiptID, receipt.Stub())
		if err != nil {
			return fmt.Errorf("could not store receipt metadata: %w", err)
		}
		err = operation.IndexExecutionReceipts(rw.Writer(), receipt.ExecutionResult.BlockID, receiptID)
		if err != nil {
			return fmt.Errorf("could not index receipt by the block it computes: %w", err)
		}
		return nil
	}

	retrieve := func(r storage.Reader, receiptID flow.Identifier) (*flow.ExecutionReceipt, error) {
		var meta flow.ExecutionReceiptStub
		err := operation.RetrieveExecutionReceiptStub(r, receiptID, &meta)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve receipt meta: %w", err)
		}
		result, err := results.ByID(meta.ResultID)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve result: %w", err)
		}
		return flow.ExecutionReceiptFromStub(meta, *result), nil
	}

	return &ExecutionReceipts{
		db:      db,
		results: results,
		cache: newCache(collector, metrics.ResourceReceipt,
			withLimit[flow.Identifier, *flow.ExecutionReceipt](cacheSize),
			withStore(store),
			withRetrieve(retrieve)),
	}
}

// storeMyReceipt assembles the operations to store an arbitrary receipt.
func (r *ExecutionReceipts) storeTx(rw storage.ReaderBatchWriter, receipt *flow.ExecutionReceipt) error {
	return r.cache.PutTx(rw, receipt.ID(), receipt)
}

func (r *ExecutionReceipts) byID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error) {
	val, err := r.cache.Get(r.db.Reader(), receiptID)
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (r *ExecutionReceipts) byBlockID(blockID flow.Identifier) ([]*flow.ExecutionReceipt, error) {
	var receiptIDs []flow.Identifier
	err := operation.LookupExecutionReceipts(r.db.Reader(), blockID, &receiptIDs)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("could not find receipt index for block: %w", err)
	}

	var receipts []*flow.ExecutionReceipt
	for _, id := range receiptIDs {
		receipt, err := r.byID(id)
		if err != nil {
			return nil, fmt.Errorf("could not find receipt with id %v: %w", id, err)
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func (r *ExecutionReceipts) Store(receipt *flow.ExecutionReceipt) error {
	return r.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return r.storeTx(rw, receipt)
	})
}

func (r *ExecutionReceipts) BatchStore(receipt *flow.ExecutionReceipt, rw storage.ReaderBatchWriter) error {
	return r.storeTx(rw, receipt)
}

func (r *ExecutionReceipts) ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error) {
	return r.byID(receiptID)
}

func (r *ExecutionReceipts) ByBlockID(blockID flow.Identifier) (flow.ExecutionReceiptList, error) {
	return r.byBlockID(blockID)
}
