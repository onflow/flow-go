package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var _ storage.LightTransactionResults = (*LightTransactionResults)(nil)

type LightTransactionResults struct {
	db         storage.DB
	cache      *Cache[string, flow.LightTransactionResult]
	indexCache *Cache[string, flow.LightTransactionResult]
	blockCache *Cache[string, []flow.LightTransactionResult]
}

func NewLightTransactionResults(collector module.CacheMetrics, db storage.DB, transactionResultsCacheSize uint) *LightTransactionResults {
	retrieve := func(r storage.Reader, key string) (flow.LightTransactionResult, error) {
		var txResult flow.LightTransactionResult
		blockID, txID, err := KeyToBlockIDTransactionID(key)
		if err != nil {
			return flow.LightTransactionResult{}, fmt.Errorf("could not convert key: %w", err)
		}

		err = operation.RetrieveLightTransactionResult(r, blockID, txID, &txResult)
		if err != nil {
			return flow.LightTransactionResult{}, err
		}
		return txResult, nil
	}
	retrieveIndex := func(r storage.Reader, key string) (flow.LightTransactionResult, error) {
		var txResult flow.LightTransactionResult
		blockID, txIndex, err := KeyToBlockIDIndex(key)
		if err != nil {
			return flow.LightTransactionResult{}, fmt.Errorf("could not convert index key: %w", err)
		}

		err = operation.RetrieveLightTransactionResultByIndex(r, blockID, txIndex, &txResult)
		if err != nil {
			return flow.LightTransactionResult{}, err
		}
		return txResult, nil
	}
	retrieveForBlock := func(r storage.Reader, key string) ([]flow.LightTransactionResult, error) {
		var txResults []flow.LightTransactionResult

		blockID, err := KeyToBlockID(key)
		if err != nil {
			return nil, fmt.Errorf("could not convert index key: %w", err)
		}

		err = operation.LookupLightTransactionResultsByBlockIDUsingIndex(r, blockID, &txResults)
		if err != nil {
			return nil, err
		}
		return txResults, nil
	}
	return &LightTransactionResults{
		db: db,
		cache: newCache(collector, metrics.ResourceTransactionResults,
			withLimit[string, flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, flow.LightTransactionResult]),
			withRetrieve(retrieve),
		),
		indexCache: newCache(collector, metrics.ResourceTransactionResultIndices,
			withLimit[string, flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, flow.LightTransactionResult]),
			withRetrieve(retrieveIndex),
		),
		blockCache: newCache(collector, metrics.ResourceTransactionResultIndices,
			withLimit[string, []flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[string, []flow.LightTransactionResult]),
			withRetrieve(retrieveForBlock),
		),
	}
}

func (tr *LightTransactionResults) BatchStore(blockID flow.Identifier, transactionResults []flow.LightTransactionResult, rw storage.ReaderBatchWriter) error {
	w := rw.Writer()

	for i, result := range transactionResults {
		err := operation.BatchInsertLightTransactionResult(w, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result: %w", err)
		}

		err = operation.BatchIndexLightTransactionResult(w, blockID, uint32(i), &result)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result: %w", err)
		}
	}

	storage.OnCommitSucceed(rw, func() {
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

func (tr *LightTransactionResults) BatchStoreBadger(blockID flow.Identifier, transactionResults []flow.LightTransactionResult, batch storage.BatchStorage) error {
	panic("LightTransactionResults BatchStoreBadger not implemented")
}

// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
func (tr *LightTransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.LightTransactionResult, error) {
	reader, err := tr.db.Reader()
	if err != nil {
		return nil, err
	}

	key := KeyFromBlockIDTransactionID(blockID, txID)
	transactionResult, err := tr.cache.Get(reader, key)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
func (tr *LightTransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error) {
	reader, err := tr.db.Reader()
	if err != nil {
		return nil, err
	}

	key := KeyFromBlockIDIndex(blockID, txIndex)
	transactionResult, err := tr.indexCache.Get(reader, key)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
func (tr *LightTransactionResults) ByBlockID(blockID flow.Identifier) ([]flow.LightTransactionResult, error) {
	reader, err := tr.db.Reader()
	if err != nil {
		return nil, err
	}

	key := KeyFromBlockID(blockID)
	transactionResults, err := tr.blockCache.Get(reader, key)
	if err != nil {
		return nil, err
	}
	return transactionResults, nil
}
