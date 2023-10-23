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

var _ storage.LightTransactionResults = (*LightTransactionResults)(nil)

type LightTransactionResults struct {
	db         *badger.DB
	cache      *Cache[string, flow.LightTransactionResult]
	indexCache *Cache[string, flow.LightTransactionResult]
	blockCache *Cache[string, []flow.LightTransactionResult]
}

func NewLightTransactionResults(collector module.CacheMetrics, db *badger.DB, transactionResultsCacheSize uint) *LightTransactionResults {
	retrieve := func(key string) func(tx *badger.Txn) (flow.LightTransactionResult, error) {
		var txResult flow.LightTransactionResult
		return func(tx *badger.Txn) (flow.LightTransactionResult, error) {

			blockID, txID, err := KeyToBlockIDTransactionID(key)
			if err != nil {
				return flow.LightTransactionResult{}, fmt.Errorf("could not convert key: %w", err)
			}

			err = operation.RetrieveLightTransactionResult(blockID, txID, &txResult)(tx)
			if err != nil {
				return flow.LightTransactionResult{}, handleError(err, flow.LightTransactionResult{})
			}
			return txResult, nil
		}
	}
	retrieveIndex := func(key string) func(tx *badger.Txn) (flow.LightTransactionResult, error) {
		var txResult flow.LightTransactionResult
		return func(tx *badger.Txn) (flow.LightTransactionResult, error) {

			blockID, txIndex, err := KeyToBlockIDIndex(key)
			if err != nil {
				return flow.LightTransactionResult{}, fmt.Errorf("could not convert index key: %w", err)
			}

			err = operation.RetrieveLightTransactionResultByIndex(blockID, txIndex, &txResult)(tx)
			if err != nil {
				return flow.LightTransactionResult{}, handleError(err, flow.LightTransactionResult{})
			}
			return txResult, nil
		}
	}
	retrieveForBlock := func(key string) func(tx *badger.Txn) ([]flow.LightTransactionResult, error) {
		var txResults []flow.LightTransactionResult
		return func(tx *badger.Txn) ([]flow.LightTransactionResult, error) {

			blockID, err := KeyToBlockID(key)
			if err != nil {
				return nil, fmt.Errorf("could not convert index key: %w", err)
			}

			err = operation.LookupLightTransactionResultsByBlockIDUsingIndex(blockID, &txResults)(tx)
			if err != nil {
				return nil, handleError(err, flow.LightTransactionResult{})
			}
			return txResults, nil
		}
	}
	return &LightTransactionResults{
		db: db,
		cache: newCache[string, flow.LightTransactionResult](collector, metrics.ResourceTransactionResults,
			withLimit[string, flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, flow.LightTransactionResult]),
			withRetrieve(retrieve),
		),
		indexCache: newCache[string, flow.LightTransactionResult](collector, metrics.ResourceTransactionResultIndices,
			withLimit[string, flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, flow.LightTransactionResult]),
			withRetrieve(retrieveIndex),
		),
		blockCache: newCache[string, []flow.LightTransactionResult](collector, metrics.ResourceTransactionResultIndices,
			withLimit[string, []flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, []flow.LightTransactionResult]),
			withRetrieve(retrieveForBlock),
		),
	}
}

func (tr *LightTransactionResults) BatchStore(blockID flow.Identifier, transactionResults []flow.LightTransactionResult, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()

	for i, result := range transactionResults {
		err := operation.BatchInsertLightTransactionResult(blockID, &result)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result: %w", err)
		}

		err = operation.BatchIndexLightTransactionResult(blockID, uint32(i), &result)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result: %w", err)
		}
	}

	batch.OnSucceed(func() {
		for i, result := range transactionResults {
			key := KeyFromBlockIDTransactionID(blockID, result.TransactionID)
			// cache for each transaction, so that it's faster to retrieve
			tr.cache.Insert(key, result)

			index := uint32(i)

			keyIndex := KeyFromBlockIDIndex(blockID, index)
			tr.indexCache.Insert(keyIndex, result)
		}

		key := KeyFromBlockID(blockID)
		tr.blockCache.Insert(key, transactionResults)
	})
	return nil
}

// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
func (tr *LightTransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.LightTransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockIDTransactionID(blockID, txID)
	transactionResult, err := tr.cache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
func (tr *LightTransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockIDIndex(blockID, txIndex)
	transactionResult, err := tr.indexCache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
func (tr *LightTransactionResults) ByBlockID(blockID flow.Identifier) ([]flow.LightTransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockID(blockID)
	transactionResults, err := tr.blockCache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return transactionResults, nil
}
