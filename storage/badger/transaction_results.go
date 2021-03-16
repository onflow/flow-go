package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type TransactionResults struct {
	db    *badger.DB
	cache *Cache
}

type BlockIDTransactionID struct {
	BlockID       flow.Identifier
	TransactionID flow.Identifier
}

func keyFromBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) string {
	return fmt.Sprintf("%x%x", blockID, txID)
}

func keyToBlockIDTransactionID(key string) (flow.Identifier, flow.Identifier, error) {
	blockIDStr := key[:32]
	txIDStr := key[32:]
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

func NewTransactionResults(collector module.CacheMetrics, db *badger.DB) *TransactionResults {
	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		var txResult flow.TransactionResult
		return func(tx *badger.Txn) (interface{}, error) {

			blockID, txID, err := keyToBlockIDTransactionID(key.(string))
			if err != nil {
				return nil, fmt.Errorf("could not convert key: %w", err)
			}

			err = db.View(operation.RetrieveTransactionResult(blockID, txID, &txResult))
			if err != nil {
				return nil, handleError(err, flow.TransactionResult{})
			}
			return txResult, nil
		}
	}
	return &TransactionResults{
		db: db,
		cache: newCache(collector,
			withLimit(10000),
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
			key := keyFromBlockIDTransactionID(blockID, result.TransactionID)
			// cache for each transaction, so that it's faster to retrieve
			tr.cache.Put(key, result)
		}
	})
	return nil
}

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
func (tr *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	val, err := tr.cache.Get(blockID)(tx)
	if err != nil {
		return nil, err
	}
	transactionResult, ok := val.(flow.TransactionResult)
	if !ok {
		return nil, fmt.Errorf("could not convert transaction result: %w", err)
	}
	return &transactionResult, nil
}
