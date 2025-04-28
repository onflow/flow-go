package store

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var _ storage.TransactionResultErrorMessages = (*TransactionResultErrorMessages)(nil)

type TransactionResultErrorMessages struct {
	db         storage.DB
	cache      *Cache[string, flow.TransactionResultErrorMessage]
	indexCache *Cache[string, flow.TransactionResultErrorMessage]
	blockCache *Cache[string, []flow.TransactionResultErrorMessage]
}

func NewTransactionResultErrorMessages(collector module.CacheMetrics, db storage.DB, transactionResultsCacheSize uint) *TransactionResultErrorMessages {
	retrieve := func(r storage.Reader, key string) (flow.TransactionResultErrorMessage, error) {
		var txResultErrMsg flow.TransactionResultErrorMessage
		blockID, txID, err := KeyToBlockIDTransactionID(key)
		if err != nil {
			return flow.TransactionResultErrorMessage{}, fmt.Errorf("could not convert key: %w", err)
		}

		err = operation.RetrieveTransactionResultErrorMessage(r, blockID, txID, &txResultErrMsg)
		if err != nil {
			return flow.TransactionResultErrorMessage{}, err
		}
		return txResultErrMsg, nil
	}
	retrieveIndex := func(r storage.Reader, key string) (flow.TransactionResultErrorMessage, error) {
		var txResultErrMsg flow.TransactionResultErrorMessage
		blockID, txIndex, err := KeyToBlockIDIndex(key)
		if err != nil {
			return flow.TransactionResultErrorMessage{}, fmt.Errorf("could not convert index key: %w", err)
		}

		err = operation.RetrieveTransactionResultErrorMessageByIndex(r, blockID, txIndex, &txResultErrMsg)
		if err != nil {
			return flow.TransactionResultErrorMessage{}, err
		}
		return txResultErrMsg, nil
	}
	retrieveForBlock := func(r storage.Reader, key string) ([]flow.TransactionResultErrorMessage, error) {
		var txResultErrMsg []flow.TransactionResultErrorMessage
		blockID, err := KeyToBlockID(key)
		if err != nil {
			return nil, fmt.Errorf("could not convert index key: %w", err)
		}

		err = operation.LookupTransactionResultErrorMessagesByBlockIDUsingIndex(r, blockID, &txResultErrMsg)
		if err != nil {
			return nil, err
		}
		return txResultErrMsg, nil
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
	return t.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return t.batchStore(blockID, transactionResultErrorMessages, rw)
	})
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

	reader, err := t.db.Reader()
	if err != nil {
		return false, err
	}

	// otherwise, check badger store
	var exists bool
	err = operation.TransactionResultErrorMessagesExists(reader, blockID, &exists)
	if err != nil {
		return false, fmt.Errorf("could not check existence: %w", err)
	}
	return exists, nil
}

// BatchStore inserts a batch of transaction result error messages into a batch
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) batchStore(
	blockID flow.Identifier,
	transactionResultErrorMessages []flow.TransactionResultErrorMessage,
	batch storage.ReaderBatchWriter,
) error {
	writer := batch.Writer()
	for _, result := range transactionResultErrorMessages {
		err := operation.BatchInsertTransactionResultErrorMessage(writer, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch insert tx result error message: %w", err)
		}

		err = operation.BatchIndexTransactionResultErrorMessage(writer, blockID, &result)
		if err != nil {
			return fmt.Errorf("cannot batch index tx result error message: %w", err)
		}
	}

	storage.OnCommitSucceed(batch, func() {
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
	reader, err := t.db.Reader()
	if err != nil {
		return nil, err
	}

	key := KeyFromBlockIDTransactionID(blockID, transactionID)
	transactionResultErrorMessage, err := t.cache.Get(reader, key)
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
	reader, err := t.db.Reader()
	if err != nil {
		return nil, err
	}

	key := KeyFromBlockIDIndex(blockID, txIndex)
	transactionResultErrorMessage, err := t.indexCache.Get(reader, key)
	if err != nil {
		return nil, err
	}
	return &transactionResultErrorMessage, nil
}

// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
// Note: This method will return an empty slice both if the block is not indexed yet and if the block does not have any errors.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) ByBlockID(blockID flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	reader, err := t.db.Reader()
	if err != nil {
		return nil, err
	}

	key := KeyFromBlockID(blockID)
	transactionResultErrorMessages, err := t.blockCache.Get(reader, key)
	if err != nil {
		return nil, err
	}
	return transactionResultErrorMessages, nil
}
