package unsynchronized

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

type TransactionResultErrorMessages struct {
	// TODO: A mutexes aren't strictly necessary here since, by design, data is written only once
	// before any future reads. However, we're keeping it temporarily during active development
	// for safety and debugging purposes. It will be removed once the implementation is finalized.
	store     map[string]flow.TransactionResultErrorMessage
	storeLock sync.RWMutex

	indexStore     map[string]flow.TransactionResultErrorMessage
	indexStoreLock sync.RWMutex

	blockStore     map[string][]flow.TransactionResultErrorMessage
	blockStoreLock sync.RWMutex
}

func NewTransactionResultErrorMessages() *TransactionResultErrorMessages {
	return &TransactionResultErrorMessages{
		store:          make(map[string]flow.TransactionResultErrorMessage),
		storeLock:      sync.RWMutex{},
		indexStore:     make(map[string]flow.TransactionResultErrorMessage),
		indexStoreLock: sync.RWMutex{},
		blockStore:     make(map[string][]flow.TransactionResultErrorMessage),
		blockStoreLock: sync.RWMutex{},
	}
}

var _ storage.TransactionResultErrorMessages = (*TransactionResultErrorMessages)(nil)

// Exists returns true if transaction result error messages for the given ID have been stored.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Exists(blockID flow.Identifier) (bool, error) {
	_, err := t.ByBlockID(blockID)
	if err != nil {
		return false, err
	}

	return true, nil
}

// ByBlockIDTransactionID returns the transaction result error message for the given block ID and transaction ID.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no transaction error message is known at given block and transaction id.
func (t *TransactionResultErrorMessages) ByBlockIDTransactionID(
	blockID flow.Identifier,
	transactionID flow.Identifier,
) (*flow.TransactionResultErrorMessage, error) {
	key := store.KeyFromBlockIDTransactionID(blockID, transactionID)

	t.storeLock.RLock()
	val, ok := t.store[key]
	t.storeLock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

// ByBlockIDTransactionIndex returns the transaction result error message for the given blockID and transaction index.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no transaction error message is known at given block and transaction index.
func (t *TransactionResultErrorMessages) ByBlockIDTransactionIndex(
	blockID flow.Identifier,
	txIndex uint32,
) (*flow.TransactionResultErrorMessage, error) {
	key := store.KeyFromBlockIDIndex(blockID, txIndex)

	t.indexStoreLock.RLock()
	val, ok := t.indexStore[key]
	t.indexStoreLock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
// Note: This method will return an empty slice both if the block is not indexed yet and if the block does not have any errors.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	key := store.KeyFromBlockID(id)

	t.blockStoreLock.RLock()
	val, ok := t.blockStore[key]
	t.blockStoreLock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// Store will store transaction result error messages for the given block ID.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Store(
	blockID flow.Identifier,
	transactionResultErrorMessages []flow.TransactionResultErrorMessage,
) error {
	key := store.KeyFromBlockID(blockID)
	t.blockStoreLock.Lock()
	t.blockStore[key] = transactionResultErrorMessages
	t.blockStoreLock.Unlock()

	for i, txResult := range transactionResultErrorMessages {
		txIDKey := store.KeyFromBlockIDTransactionID(blockID, txResult.TransactionID)
		txIndexKey := store.KeyFromBlockIDIndex(blockID, uint32(i))

		t.storeLock.Lock()
		t.store[txIDKey] = txResult
		t.storeLock.Unlock()

		t.indexStoreLock.Lock()
		t.indexStore[txIndexKey] = txResult
		t.indexStoreLock.Unlock()
	}

	return nil
}
