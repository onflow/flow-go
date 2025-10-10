package store

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var _ storage.LightTransactionResults = (*LightTransactionResults)(nil)

type LightTransactionResults struct {
	db         storage.DB
	cache      *Cache[TwoIdentifier, flow.LightTransactionResult]       // Key: blockID + txID
	indexCache *Cache[IdentifierAndUint32, flow.LightTransactionResult] // Key: blockID + txIndex
	blockCache *Cache[flow.Identifier, []flow.LightTransactionResult]   // Key: blockID
}

func NewLightTransactionResults(collector module.CacheMetrics, db storage.DB, transactionResultsCacheSize uint) *LightTransactionResults {
	retrieve := func(r storage.Reader, key TwoIdentifier) (flow.LightTransactionResult, error) {
		blockID, txID := KeyToBlockIDTransactionID(key)

		var txResult flow.LightTransactionResult
		err := operation.RetrieveLightTransactionResult(r, blockID, txID, &txResult)
		if err != nil {
			return flow.LightTransactionResult{}, err
		}
		return txResult, nil
	}

	retrieveIndex := func(r storage.Reader, key IdentifierAndUint32) (flow.LightTransactionResult, error) {
		blockID, txIndex := KeyToBlockIDIndex(key)

		var txResult flow.LightTransactionResult
		err := operation.RetrieveLightTransactionResultByIndex(r, blockID, txIndex, &txResult)
		if err != nil {
			return flow.LightTransactionResult{}, err
		}
		return txResult, nil
	}

	retrieveForBlock := func(r storage.Reader, blockID flow.Identifier) ([]flow.LightTransactionResult, error) {
		var txResults []flow.LightTransactionResult
		err := operation.LookupLightTransactionResultsByBlockIDUsingIndex(r, blockID, &txResults)
		if err != nil {
			return nil, err
		}
		return txResults, nil
	}

	return &LightTransactionResults{
		db: db,
		cache: newCache(collector, metrics.ResourceTransactionResults,
			withLimit[TwoIdentifier, flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[TwoIdentifier, flow.LightTransactionResult]),
			withRetrieve(retrieve),
		),
		indexCache: newCache(collector, metrics.ResourceTransactionResultIndices,
			withLimit[IdentifierAndUint32, flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[IdentifierAndUint32, flow.LightTransactionResult]),
			withRetrieve(retrieveIndex),
		),
		blockCache: newCache(collector, metrics.ResourceTransactionResultIndices,
			withLimit[flow.Identifier, []flow.LightTransactionResult](transactionResultsCacheSize),
			withStore(noopStore[flow.Identifier, []flow.LightTransactionResult]),
			withRetrieve(retrieveForBlock),
		),
	}
}

func (tr *LightTransactionResults) BatchStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, transactionResults []flow.LightTransactionResult) error {
	// requires [storage.LockInsertLightTransactionResult]
	err := operation.InsertAndIndexLightTransactionResults(lctx, rw, blockID, transactionResults)
	if err != nil {
		return err
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

		tr.blockCache.Insert(blockID, transactionResults)
	})
	return nil
}

func (tr *LightTransactionResults) BatchStoreBadger(blockID flow.Identifier, transactionResults []flow.LightTransactionResult, batch storage.BatchStorage) error {
	panic("LightTransactionResults BatchStoreBadger not implemented")
}

// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
func (tr *LightTransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.LightTransactionResult, error) {
	key := KeyFromBlockIDTransactionID(blockID, txID)
	transactionResult, err := tr.cache.Get(tr.db.Reader(), key)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
func (tr *LightTransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error) {
	key := KeyFromBlockIDIndex(blockID, txIndex)
	transactionResult, err := tr.indexCache.Get(tr.db.Reader(), key)
	if err != nil {
		return nil, err
	}
	return &transactionResult, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
func (tr *LightTransactionResults) ByBlockID(blockID flow.Identifier) ([]flow.LightTransactionResult, error) {
	transactionResults, err := tr.blockCache.Get(tr.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return transactionResults, nil
}
