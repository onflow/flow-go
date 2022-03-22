package badger

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type TransactionResults struct {
	db         *badger.DB
	cache      *Cache
	indexCache *Cache
}

func KeyFromBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) string {
	return fmt.Sprintf("%x%x", blockID, txID)
}

func KeyFromBlockIDIndex(blockID flow.Identifier, txIndex uint32) string {
	idData := make([]byte, 4) //uint32 fits into 4 bytes
	binary.BigEndian.PutUint32(idData, txIndex)
	return fmt.Sprintf("%x%x", blockID, idData)
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

func KeyToBlockIDIndex(key string) (flow.Identifier, uint32, error) {
	blockIDStr := key[:64]
	indexStr := key[64:]
	blockID, err := flow.HexStringToIdentifier(blockIDStr)
	if err != nil {
		return flow.ZeroID, 0, fmt.Errorf("could not get block ID: %w", err)
	}

	txIndexBytes, err := hex.DecodeString(indexStr)
	if err != nil {
		return flow.ZeroID, 0, fmt.Errorf("could not get transaction index: %w", err)
	}
	if len(txIndexBytes) != 4 {
		return flow.ZeroID, 0, fmt.Errorf("could not get transaction index - invalid length: %d", len(txIndexBytes))
	}

	txIndex := binary.BigEndian.Uint32(txIndexBytes)

	return blockID, txIndex, nil
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
	retrieveIndex := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		var txResult flow.TransactionResult
		return func(tx *badger.Txn) (interface{}, error) {

			blockID, txIndex, err := KeyToBlockIDIndex(key.(string))
			if err != nil {
				return nil, fmt.Errorf("could not convert index key: %w", err)
			}

			err = operation.RetrieveTransactionResultByIndex(blockID, txIndex, &txResult)(tx)
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
			withRetrieve(retrieve),
		),
		indexCache: newCache(collector, metrics.ResourceTransactionResultIndices,
			withLimit(transactionResultsCacheSize),
			withStore(noopStore),
			withRetrieve(retrieveIndex),
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

// ByBlockIDIndex returns the runtime transaction result for the given block ID and transaction index
func (tr *TransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockIDIndex(blockID, txIndex)
	val, err := tr.indexCache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	transactionResult, ok := val.(flow.TransactionResult)
	if !ok {
		return nil, fmt.Errorf("could not convert transaction result: %w", err)
	}
	return &transactionResult, nil
}
