package unsynchronized

import (
	"errors"
	"fmt"
	"sync"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

type TransactionResultErrorMessages struct {
	// TODO: A mutex isn't strictly necessary here since, by design, data is written only once
	// before any future reads. However, we're keeping it temporarily during active development
	// for safety and debugging purposes. It will be removed once the implementation is finalized.
	lock       sync.RWMutex
	store      map[store.TwoIdentifier]*flow.TransactionResultErrorMessage       // Key: blockID + txID
	indexStore map[store.IdentifierAndUint32]*flow.TransactionResultErrorMessage // Key: blockID + txIndex
	blockStore map[flow.Identifier][]flow.TransactionResultErrorMessage          // Key: blockID
}

var _ storage.TransactionResultErrorMessages = (*TransactionResultErrorMessages)(nil)

func NewTransactionResultErrorMessages() *TransactionResultErrorMessages {
	return &TransactionResultErrorMessages{
		store:      make(map[store.TwoIdentifier]*flow.TransactionResultErrorMessage),
		indexStore: make(map[store.IdentifierAndUint32]*flow.TransactionResultErrorMessage),
		blockStore: make(map[flow.Identifier][]flow.TransactionResultErrorMessage),
	}
}

// Exists returns true if transaction result error messages for the given ID have been stored.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Exists(blockID flow.Identifier) (bool, error) {
	_, err := t.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil
		}

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
	t.lock.RLock()
	defer t.lock.RUnlock()

	val, ok := t.store[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
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
	t.lock.RLock()
	defer t.lock.RUnlock()

	val, ok := t.indexStore[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockID gets all transaction result error messages for a block, ordered by transaction index.
// Note: This method will return an empty slice both if the block is not indexed yet and if the block does not have any errors.
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if no block was found.
func (t *TransactionResultErrorMessages) ByBlockID(id flow.Identifier) ([]flow.TransactionResultErrorMessage, error) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	val, ok := t.blockStore[id]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// Store will store transaction result error messages for the given block ID.
//
// No errors are expected during normal operation.
func (t *TransactionResultErrorMessages) Store(
	lctx lockctx.Proof,
	blockID flow.Identifier,
	transactionResultErrorMessages []flow.TransactionResultErrorMessage,
) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.blockStore[blockID] = transactionResultErrorMessages
	for i, txResult := range transactionResultErrorMessages {
		txIDKey := store.KeyFromBlockIDTransactionID(blockID, txResult.TransactionID)
		txIndexKey := store.KeyFromBlockIDIndex(blockID, uint32(i))

		t.store[txIDKey] = &txResult
		t.indexStore[txIndexKey] = &txResult
	}

	return nil
}

// Data returns a copy of all stored transaction result error messages keyed by block ID.
func (t *TransactionResultErrorMessages) Data() []flow.TransactionResultErrorMessage {
	t.lock.RLock()
	defer t.lock.RUnlock()

	out := make([]flow.TransactionResultErrorMessage, 0, len(t.blockStore))
	for _, errorMessages := range t.blockStore {
		out = append(out, errorMessages...)
	}
	return out
}

// BatchStore inserts a batch of transaction result error messages into a batch
// This method is not implemented and will always return an error.
func (t *TransactionResultErrorMessages) BatchStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, transactionResultErrorMessages []flow.TransactionResultErrorMessage) error {
	return fmt.Errorf("not implemented")
}
