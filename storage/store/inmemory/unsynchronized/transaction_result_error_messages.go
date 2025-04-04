package unsynchronized

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

type TransactionResultErrorMessages struct {
	//TODO: we don't need a mutex here as we have a guarantee by design
	// that we write data only once and it happens before the future reads.
	// We decided to leave a mutex for some time during active development.
	// It'll be removed later on.
	lock       sync.RWMutex
	store      map[string]flow.TransactionResultErrorMessage
	indexStore map[string]flow.TransactionResultErrorMessage
	blockStore map[string][]flow.TransactionResultErrorMessage
}

func NewTransactionResultErrorMessages() *TransactionResultErrorMessages {
	return &TransactionResultErrorMessages{
		store:      make(map[string]flow.TransactionResultErrorMessage),
		indexStore: make(map[string]flow.TransactionResultErrorMessage),
		blockStore: make(map[string][]flow.TransactionResultErrorMessage),
	}
}

var _ storage.TransactionResultErrorMessagesReader = (*TransactionResultErrorMessages)(nil)

func (t *TransactionResultErrorMessages) Exists(blockID flow.Identifier) (bool, error) {
	_, err := t.ByBlockID(blockID)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (t *TransactionResultErrorMessages) ByBlockIDTransactionID(
	blockID flow.Identifier,
	transactionID flow.Identifier,
) (*flow.TransactionResultErrorMessage, error) {
	key := store.KeyFromBlockIDTransactionID(blockID, transactionID)

	t.lock.RLock()
	val, ok := t.store[key]
	t.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

func (t *TransactionResultErrorMessages) ByBlockIDTransactionIndex(
	blockID flow.Identifier,
	txIndex uint32,
) (*flow.TransactionResultErrorMessage, error) {
	key := store.KeyFromBlockIDIndex(blockID, txIndex)

	t.lock.RLock()
	val, ok := t.indexStore[key]
	t.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

func (t *TransactionResultErrorMessages) ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	key := store.KeyFromBlockID(id)

	t.lock.RLock()
	val, ok := t.blockStore[key]
	t.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

func (t *TransactionResultErrorMessages) Store(
	blockID flow.Identifier,
	transactionResultErrorMessages []flow.TransactionResultErrorMessage,
) error {
	key := store.KeyFromBlockID(blockID)
	t.lock.Lock()
	t.blockStore[key] = transactionResultErrorMessages
	t.lock.Unlock()

	for i, txResult := range transactionResultErrorMessages {
		txIDKey := store.KeyFromBlockIDTransactionID(blockID, txResult.TransactionID)
		txIndexKey := store.KeyFromBlockIDIndex(blockID, uint32(i))

		t.lock.Lock()
		t.store[txIDKey] = txResult
		t.indexStore[txIndexKey] = txResult
		t.lock.Unlock()
	}

	return nil
}

func (t *TransactionResultErrorMessages) AddToBatch(batch storage.ReaderBatchWriter) error {
	//TODO implement me
	panic("implement me")
	return nil
}
