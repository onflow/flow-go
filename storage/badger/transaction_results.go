package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type TransactionResults struct {
	db    *badger.DB
	cache *Cache
}

func KeyFromBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) string {
	return fmt.Sprintf("%x%x", blockID, txID)
}

func KeyToBlockIDTransactionID(key string) (flow.Identifier, flow.Identifier, error) {
	blockIDStr := key[:64]
	txIDStr := key[64:]
	blockID, err := flow.HexStringToIdentifier(blockIDStr)
	if err != nil {
		return flow.ZeroID, flow.ZeroID, fmt.Errorf("could not get block ID: %w", err)
	}

	txID, err := flow.HexStringToIdentifier(txIDStr)
	if err != nil {
		return flow.ZeroID, flow.ZeroID, fmt.Errorf("could not get transaction id: %w", err)
	}

	return blockID, txID, nil
}

func NewTransactionResults(collector module.CacheMetrics, db *badger.DB, transactionResultsCacheSize uint) *TransactionResults {
	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		var txResult flow.TransactionResult
		return func(tx *badger.Txn) (interface{}, error) {

			blockID, txID, err := KeyToBlockIDTransactionID(key.(string))
			if err != nil {
				return nil, fmt.Errorf("could not convert key: %w", err)
			}

			err = operation.RetrieveTransactionResult(blockID, txID, &txResult)(tx)
			if err != nil {
				return nil, handleError(err, flow.TransactionResult{})
			}
			return txResult, nil
		}
	}
	return &TransactionResults{
		db: db,
		cache: newCache(collector, metrics.ResourceTransactionResults,
			withLimit(transactionResultsCacheSize),
			withStore(noopStore),
			withRetrieve(retrieve)),
	}
}

// BatchStore will store the transaction results for the given block ID in a batch
func (tr *TransactionResults) BatchStore(blockID flow.Identifier, transactionResults []flow.TransactionResult, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()

	for _, result := range transactionResults {
		err := operation.BatchInsertTransactionResult(blockID, &result)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result: %w", err)
		}
	}

	batch.OnSucceed(func() {
		for _, result := range transactionResults {
			key := KeyFromBlockIDTransactionID(blockID, result.TransactionID)
			// cache for each transaction, so that it's faster to retrieve
			tr.cache.Insert(key, result)
		}
	})
	return nil
}

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
func (tr *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockIDTransactionID(blockID, txID)
	val, err := tr.cache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	transactionResult, ok := val.(flow.TransactionResult)
	if !ok {
		return nil, fmt.Errorf("could not convert transaction result: %w", err)
	}
	return &transactionResult, nil
}

// RemoveByBlockID removes transaction results by block ID
func (tr *TransactionResults) RemoveByBlockID(blockID flow.Identifier) error {
	return tr.db.Update(operation.RemoveTransactionResultsByBlockID(blockID))
}
