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

var _ storage.TransactionResultErrorMessages = (*TransactionResultErrorMessages)(nil)

type TransactionResultErrorMessages struct {
	db         *badger.DB
	cache      *Cache[string, flow.TransactionResultErrorMessage]
	indexCache *Cache[string, flow.TransactionResultErrorMessage]
	blockCache *Cache[string, []flow.TransactionResultErrorMessage]
}

func NewTransactionResultErrorMessages(collector module.CacheMetrics, db *badger.DB, transactionResultsCacheSize uint) *TransactionResultErrorMessages {
	retrieve := func(key string) func(tx *badger.Txn) (flow.TransactionResultErrorMessage, error) {
		var txResultErrMsg flow.TransactionResultErrorMessage
		return func(tx *badger.Txn) (flow.TransactionResultErrorMessage, error) {

			blockID, txID, err := KeyToBlockIDTransactionID(key)
			if err != nil {
				return flow.TransactionResultErrorMessage{}, fmt.Errorf("could not convert key: %w", err)
			}

			err = operation.RetrieveTransactionResultErrorMessage(blockID, txID, &txResultErrMsg)(tx)
			if err != nil {
				return flow.TransactionResultErrorMessage{}, handleError(err, flow.TransactionResultErrorMessage{})
			}
			return txResultErrMsg, nil
		}
	}
	retrieveIndex := func(key string) func(tx *badger.Txn) (flow.TransactionResultErrorMessage, error) {
		var txResultErrMsg flow.TransactionResultErrorMessage
		return func(tx *badger.Txn) (flow.TransactionResultErrorMessage, error) {

			blockID, txIndex, err := KeyToBlockIDIndex(key)
			if err != nil {
				return flow.TransactionResultErrorMessage{}, fmt.Errorf("could not convert index key: %w", err)
			}

			err = operation.RetrieveTransactionResultErrorMessageByIndex(blockID, txIndex, &txResultErrMsg)(tx)
			if err != nil {
				return flow.TransactionResultErrorMessage{}, handleError(err, flow.TransactionResultErrorMessage{})
			}
			return txResultErrMsg, nil
		}
	}
	retrieveForBlock := func(key string) func(tx *badger.Txn) ([]flow.TransactionResultErrorMessage, error) {
		var txResultErrMsg []flow.TransactionResultErrorMessage
		return func(tx *badger.Txn) ([]flow.TransactionResultErrorMessage, error) {

			blockID, err := KeyToBlockID(key)
			if err != nil {
				return nil, fmt.Errorf("could not convert index key: %w", err)
			}

			err = operation.LookupTransactionResultErrorMessagesByBlockIDUsingIndex(blockID, &txResultErrMsg)(tx)
			if err != nil {
				return nil, handleError(err, flow.TransactionResultErrorMessage{})
			}
			return txResultErrMsg, nil
		}
	}

	return &TransactionResultErrorMessages{
		db: db,
		cache: newCache[string, flow.TransactionResultErrorMessage](collector, metrics.ResourceTransactionResultErrorMessages,
			withLimit[string, flow.TransactionResultErrorMessage](transactionResultsCacheSize),
			withStore(noopStore[string, flow.TransactionResultErrorMessage]),
			withRetrieve(retrieve),
		),
		indexCache: newCache[string, flow.TransactionResultErrorMessage](collector, metrics.ResourceTransactionResultErrorMessagesIndices,
			withLimit[string, flow.TransactionResultErrorMessage](transactionResultsCacheSize),
			withStore(noopStore[string, flow.TransactionResultErrorMessage]),
			withRetrieve(retrieveIndex),
		),
		blockCache: newCache[string, []flow.TransactionResultErrorMessage](collector, metrics.ResourceTransactionResultErrorMessagesIndices,
			withLimit[string, []flow.TransactionResultErrorMessage](transactionResultsCacheSize),
			withStore(noopStore[string, []flow.TransactionResultErrorMessage]),
			withRetrieve(retrieveForBlock),
		),
	}
}

// Store will store transaction result error messages for the given block ID.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Store(blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error {
	batch := NewBatch(t.db)

	err := t.batchStore(blockID, transactionResultErrorMessages, batch)
	if err != nil {
		return err
	}

	err = batch.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush batch: %w", err)
	}

	return nil
}

// Exists returns true if transaction result error messages for the given ID have been stored.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Exists(blockID flow.Identifier) (bool, error) {
	// if the block is in the cache, return true
	key := KeyFromBlockID(blockID)
	if ok := t.blockCache.IsCached(key); ok {
		return ok, nil
	}
	// otherwise, check badger store
	var exists bool
	err := t.db.View(operation.TransactionResultErrorMessagesExists(blockID, &exists))
	if err != nil {
		return false, fmt.Errorf("could not check existence: %w", err)
	}
	return exists, nil
}

// BatchStore inserts a batch of transaction result error messages into a batch
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) batchStore(blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()

	for _, result := range transactionResultErrorMessages {
		err := operation.BatchInsertTransactionResultErrorMessage(blockID, &result)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result error message: %w", err)
		}

		err = operation.BatchIndexTransactionResultErrorMessage(blockID, &result)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result error message: %w", err)
		}
	}

	batch.OnSucceed(func() {
		for _, result := range transactionResultErrorMessages {
			key := KeyFromBlockIDTransactionID(blockID, result.TransactionID)
			// cache for each transaction, so that it's faster to retrieve
			t.cache.Insert(key, result)

			keyIndex := KeyFromBlockIDIndex(blockID, result.Index)
			t.indexCache.Insert(keyIndex, result)
		}

		key := KeyFromBlockID(blockID)
		t.blockCache.Insert(key, transactionResultErrorMessages)
	})
	return nil
}

// ByBlockIDTransactionID returns the transaction result error message for the given block ID and transaction ID
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no transaction error message is known at given block and transaction id.
func (t *TransactionResultErrorMessages) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResultErrorMessage, error) {
	tx := t.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockIDTransactionID(blockID, transactionID)
	transactionResultErrorMessage, err := t.cache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return &transactionResultErrorMessage, nil
}

// ByBlockIDTransactionIndex returns the transaction result error message for the given blockID and transaction index
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no transaction error message is known at given block and transaction index.
func (t *TransactionResultErrorMessages) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.TransactionResultErrorMessage, error) {
	tx := t.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockIDIndex(blockID, txIndex)
	transactionResultErrorMessage, err := t.indexCache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return &transactionResultErrorMessage, nil
}

// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no transaction error messages is known at given block.
func (t *TransactionResultErrorMessages) ByBlockID(blockID flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	tx := t.db.NewTransaction(false)
	defer tx.Discard()
	key := KeyFromBlockID(blockID)
	transactionResultErrorMessages, err := t.blockCache.Get(key)(tx)
	if err != nil {
		return nil, err
	}
	return transactionResultErrorMessages, nil
}
