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

var _ storage.TransactionResults = (*TransactionResults)(nil)

type TransactionResults struct {
	db         *badger.DB
	cache      *Cache[string, flow.TransactionResult]
	indexCache *Cache[string, flow.TransactionResult]
	blockCache *Cache[string, []flow.TransactionResult]
}

func NewTransactionResults(collector module.CacheMetrics, db *badger.DB, transactionResultsCacheSize uint) *TransactionResults {
	retrieve := func(key string) func(tx *badger.Txn) (flow.TransactionResult, error) {
		var txResult flow.TransactionResult
		return func(tx *badger.Txn) (flow.TransactionResult, error) {

			blockID, txID, err := storage.KeyToBlockIDTransactionID(key)
			if err != nil {
				return flow.TransactionResult{}, fmt.Errorf("could not convert key: %w", err)
			}

			err = operation.RetrieveTransactionResult(blockID, txID, &txResult)(tx)
			if err != nil {
				return flow.TransactionResult{}, handleError(err, flow.TransactionResult{})
			}
			return txResult, nil
		}
	}
	retrieveIndex := func(key string) func(tx *badger.Txn) (flow.TransactionResult, error) {
		var txResult flow.TransactionResult
		return func(tx *badger.Txn) (flow.TransactionResult, error) {

			blockID, txIndex, err := storage.KeyToBlockIDIndex(key)
			if err != nil {
				return flow.TransactionResult{}, fmt.Errorf("could not convert index key: %w", err)
			}

			err = operation.RetrieveTransactionResultByIndex(blockID, txIndex, &txResult)(tx)
			if err != nil {
				return flow.TransactionResult{}, handleError(err, flow.TransactionResult{})
			}
			return txResult, nil
		}
	}
	retrieveForBlock := func(key string) func(tx *badger.Txn) ([]flow.TransactionResult, error) {
		var txResults []flow.TransactionResult
		return func(tx *badger.Txn) ([]flow.TransactionResult, error) {

			blockID, err := storage.KeyToBlockID(key)
			if err != nil {
				return nil, fmt.Errorf("could not convert index key: %w", err)
			}

			err = operation.LookupTransactionResultsByBlockIDUsingIndex(blockID, &txResults)(tx)
			if err != nil {
				return nil, handleError(err, flow.TransactionResult{})
			}
			return txResults, nil
		}
	}
	return &TransactionResults{
		db: db,
		cache: newCache[string, flow.TransactionResult](collector, metrics.ResourceTransactionResults,
			withLimit[string, flow.TransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, flow.TransactionResult]),
			withRetrieve(retrieve),
		),
		indexCache: newCache[string, flow.TransactionResult](collector, metrics.ResourceTransactionResultIndices,
			withLimit[string, flow.TransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, flow.TransactionResult]),
			withRetrieve(retrieveIndex),
		),
		blockCache: newCache[string, []flow.TransactionResult](collector, metrics.ResourceTransactionResultIndices,
			withLimit[string, []flow.TransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, []flow.TransactionResult]),
			withRetrieve(retrieveForBlock),
		),
	}
}

// BatchStore will store the transaction results for the given block ID in a batch
func (tr *TransactionResults) BatchStore(blockID flow.Identifier, transactionResults []flow.TransactionResult, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()

	for i, result := range transactionResults {
		err := operation.BatchInsertTransactionResult(blockID, &result)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result: %w", err)
		}

		err = operation.BatchIndexTransactionResult(blockID, uint32(i), &result)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result: %w", err)
		}
	}

	batch.OnSucceed(func() {
		for i, result := range transactionResults {
			key := storage.KeyFromBlockIDTransactionID(blockID, result.TransactionID)
			// cache for each transaction, so that it's faster to retrieve
			tr.cache.Insert(key, result)

			index := uint32(i)

			keyIndex := storage.KeyFromBlockIDIndex(blockID, index)
			tr.indexCache.Insert(keyIndex, result)
		}

		key := storage.KeyFromBlockID(blockID)
		tr.blockCache.Insert(key, transactionResults)
	})
	return nil
}

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
func (tr *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := storage.KeyFromBlockIDTransactionID(blockID, txID)
	transactionResult, err := tr.cache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockIDTransactionIndex returns the runtime transaction result for the given block ID and transaction index
func (tr *TransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := storage.KeyFromBlockIDIndex(blockID, txIndex)
	transactionResult, err := tr.indexCache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
func (tr *TransactionResults) ByBlockID(blockID flow.Identifier) ([]flow.TransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := storage.KeyFromBlockID(blockID)
	transactionResults, err := tr.blockCache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return transactionResults, nil
}

// RemoveByBlockID removes transaction results by block ID
func (tr *TransactionResults) RemoveByBlockID(blockID flow.Identifier) error {
	return tr.db.Update(operation.RemoveTransactionResultsByBlockID(blockID))
}

// BatchRemoveByBlockID batch removes transaction results by block ID
func (tr *TransactionResults) BatchRemoveByBlockID(blockID flow.Identifier, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	return tr.db.View(operation.BatchRemoveTransactionResultsByBlockID(blockID, writeBatch))
}
