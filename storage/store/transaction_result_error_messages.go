package store

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

var _ storage.TransactionResultErrorMessages = (*TransactionResultErrorMessages)(nil)

type TransactionResultErrorMessages struct {
	db         storage.DB
	cache      *Cache[TwoIdentifier, flow.TransactionResultErrorMessage]       // Key: blockID + txID
	indexCache *Cache[IdentifierAndUint32, flow.TransactionResultErrorMessage] // Key: blockID + txIndex
	blockCache *Cache[flow.Identifier, []flow.TransactionResultErrorMessage]   // Key: blockID
}

func NewTransactionResultErrorMessages(collector module.CacheMetrics, db storage.DB, transactionResultsCacheSize uint) *TransactionResultErrorMessages {
	retrieve := func(r storage.Reader, key TwoIdentifier) (flow.TransactionResultErrorMessage, error) {
		blockID, txID := KeyToBlockIDTransactionID(key)

		var txResultErrMsg flow.TransactionResultErrorMessage
		err := operation.RetrieveTransactionResultErrorMessage(r, blockID, txID, &txResultErrMsg)
		if err != nil {
			return flow.TransactionResultErrorMessage{}, err
		}
		return txResultErrMsg, nil
	}

	retrieveIndex := func(r storage.Reader, key IdentifierAndUint32) (flow.TransactionResultErrorMessage, error) {
		blockID, txIndex := KeyToBlockIDIndex(key)

		var txResultErrMsg flow.TransactionResultErrorMessage
		err := operation.RetrieveTransactionResultErrorMessageByIndex(r, blockID, txIndex, &txResultErrMsg)
		if err != nil {
			return flow.TransactionResultErrorMessage{}, err
		}
		return txResultErrMsg, nil
	}

	retrieveForBlock := func(r storage.Reader, blockID flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
		var txResultErrMsg []flow.TransactionResultErrorMessage
		err := operation.LookupTransactionResultErrorMessagesByBlockIDUsingIndex(r, blockID, &txResultErrMsg)
		if err != nil {
			return nil, err
		}
		return txResultErrMsg, nil
	}

	return &TransactionResultErrorMessages{
		db: db,
		cache: newCache(collector, metrics.ResourceTransactionResultErrorMessages,
			withLimit[TwoIdentifier, flow.TransactionResultErrorMessage](transactionResultsCacheSize),
			withStore(noopStore[TwoIdentifier, flow.TransactionResultErrorMessage]),
			withRetrieve(retrieve),
		),
		indexCache: newCache(collector, metrics.ResourceTransactionResultErrorMessagesIndices,
			withLimit[IdentifierAndUint32, flow.TransactionResultErrorMessage](transactionResultsCacheSize),
			withStore(noopStore[IdentifierAndUint32, flow.TransactionResultErrorMessage]),
			withRetrieve(retrieveIndex),
		),
		blockCache: newCache(collector, metrics.ResourceTransactionResultErrorMessagesIndices,
			withLimit[flow.Identifier, []flow.TransactionResultErrorMessage](transactionResultsCacheSize),
			withStore(noopStore[flow.Identifier, []flow.TransactionResultErrorMessage]),
			withRetrieve(retrieveForBlock),
		),
	}
}

// Store will store transaction result error messages for the given block ID.
//
// the caller must hold [storage.LockInsertTransactionResultErrMessage] lock
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Store(lctx lockctx.Proof, blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error {
	return t.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return t.BatchStore(lctx, rw, blockID, transactionResultErrorMessages)
	})
}

// Exists returns true if transaction result error messages for the given ID have been stored.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Exists(blockID flow.Identifier) (bool, error) {
	// if the block is in the cache, return true
	if ok := t.blockCache.IsCached(blockID); ok {
		return ok, nil
	}

	// otherwise, check badger store
	var exists bool
	err := operation.TransactionResultErrorMessagesExists(t.db.Reader(), blockID, &exists)
	if err != nil {
		return false, fmt.Errorf("could not check existence: %w", err)
	}
	return exists, nil
}

// BatchStore inserts a batch of transaction result error messages into a batch
// the caller must hold [storage.LockInsertTransactionResultErrMessage] lock
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) BatchStore(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	blockID flow.Identifier,
	transactionResultErrorMessages []flow.TransactionResultErrorMessage,
) error {
	// requires [storage.LockInsertTransactionResultErrMessage]
	err := operation.BatchInsertAndIndexTransactionResultErrorMessages(lctx, rw, blockID, transactionResultErrorMessages)
	if err != nil {
		return err
	}

	storage.OnCommitSucceed(rw, func() {
		for _, result := range transactionResultErrorMessages {
			key := KeyFromBlockIDTransactionID(blockID, result.TransactionID)
			// cache for each transaction, so that it's faster to retrieve
			t.cache.Insert(key, result)

			keyIndex := KeyFromBlockIDIndex(blockID, result.Index)
			t.indexCache.Insert(keyIndex, result)
		}

		t.blockCache.Insert(blockID, transactionResultErrorMessages)
	})
	return nil
}

// ByBlockIDTransactionID returns the transaction result error message for the given block ID and transaction ID
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no transaction error message is known at given block and transaction id.
func (t *TransactionResultErrorMessages) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.TransactionResultErrorMessage, error) {
	key := KeyFromBlockIDTransactionID(blockID, transactionID)
	transactionResultErrorMessage, err := t.cache.Get(t.db.Reader(), key)
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
	key := KeyFromBlockIDIndex(blockID, txIndex)
	transactionResultErrorMessage, err := t.indexCache.Get(t.db.Reader(), key)
	if err != nil {
		return nil, err
	}
	return &transactionResultErrorMessage, nil
}

// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
// Note: This method will return an empty slice both if the block is not indexed yet and if the block does not have any errors.
//
// No errors are expected during normal operations.
func (t *TransactionResultErrorMessages) ByBlockID(blockID flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	transactionResultErrorMessages, err := t.blockCache.Get(t.db.Reader(), blockID)
	if err != nil {
		return nil, err
	}
	return transactionResultErrorMessages, nil
}
