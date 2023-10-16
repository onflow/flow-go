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

var _ storage.TransactionResults = (*TransactionResults)(nil)

type TransactionResults struct {
	db         *badger.DB
	cache      *Cache[string, flow.TransactionResult]
	indexCache *Cache[string, flow.TransactionResult]
	blockCache *Cache[string, []flow.TransactionResult]
}

func KeyFromBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) string {
	return fmt.Sprintf("%x%x", blockID, txID)
}

func KeyFromBlockIDIndex(blockID flow.Identifier, txIndex uint32) string {
	idData := make([]byte, 4) //uint32 fits into 4 bytes
	binary.BigEndian.PutUint32(idData, txIndex)
	return fmt.Sprintf("%x%x", blockID, idData)
}

func KeyFromBlockID(blockID flow.Identifier) string {
	return blockID.String()
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

func KeyToBlockID(key string) (flow.Identifier, error) {

	blockID, err := flow.HexStringToIdentifier(key)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get block ID: %w", err)
	}

	return blockID, err
}

func NewTransactionResults(collector module.CacheMetrics, db *badger.DB, transactionResultsCacheSize uint) *TransactionResults {
	retrieve := func(key string) func(tx *badger.Txn) (flow.TransactionResult, error) {
		var txResult flow.TransactionResult
		return func(tx *badger.Txn) (flow.TransactionResult, error) {

			blockID, txID, err := KeyToBlockIDTransactionID(key)
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

			blockID, txIndex, err := KeyToBlockIDIndex(key)
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

			blockID, err := KeyToBlockID(key)
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

// ByBlockIDTransactionID returns the runtime transaction result for the given block ID and transaction ID
func (tr *TransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, txID flow.Identifier) (*flow.TransactionResult, error) {
	tx := tr.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockIDTransactionID(blockID, txID)
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
	key := KeyFromBlockIDIndex(blockID, txIndex)
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
	key := KeyFromBlockID(blockID)
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
