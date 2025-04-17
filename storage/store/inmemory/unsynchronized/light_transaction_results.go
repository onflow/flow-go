package unsynchronized

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

type LightTransactionResults struct {
	// TODO: A mutex isn't strictly necessary here since, by design, data is written only once
	// before any future reads. However, we're keeping it temporarily during active development
	// for safety and debugging purposes. It will be removed once the implementation is finalized.
	lock       sync.RWMutex
	store      map[string]*flow.LightTransactionResult
	indexStore map[string]*flow.LightTransactionResult
	blockStore map[string][]flow.LightTransactionResult
}

var _ storage.LightTransactionResults = (*LightTransactionResults)(nil)

func NewLightTransactionResults() *LightTransactionResults {
	return &LightTransactionResults{
		store:      make(map[string]*flow.LightTransactionResult),
		indexStore: make(map[string]*flow.LightTransactionResult),
		blockStore: make(map[string][]flow.LightTransactionResult),
	}
}

// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if light transaction result at given blockID wasn't found.
func (l *LightTransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.LightTransactionResult, error) {
	key := store.KeyFromBlockIDTransactionID(blockID, transactionID)
	l.lock.RLock()
	defer l.lock.RUnlock()

	val, ok := l.store[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if light transaction result at given blockID and txIndex wasn't found.
func (l *LightTransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error) {
	key := store.KeyFromBlockIDIndex(blockID, txIndex)
	l.lock.RLock()
	defer l.lock.RUnlock()

	val, ok := l.indexStore[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
//
// Expected errors during normal operation:
//   - `storage.ErrNotFound` if light transaction results at given blockID weren't found.
func (l *LightTransactionResults) ByBlockID(id flow.Identifier) ([]flow.LightTransactionResult, error) {
	key := store.KeyFromBlockID(id)
	l.lock.RLock()
	defer l.lock.RUnlock()

	val, ok := l.blockStore[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// Store inserts a transaction results into a storage
// No errors are expected during normal operation.
func (l *LightTransactionResults) Store(blockID flow.Identifier, transactionResults []flow.LightTransactionResult) error {
	key := store.KeyFromBlockID(blockID)
	l.lock.Lock()
	defer l.lock.Unlock()

	l.blockStore[key] = transactionResults
	for i, txResult := range transactionResults {
		txIDKey := store.KeyFromBlockIDTransactionID(blockID, txResult.TransactionID)
		txIndexKey := store.KeyFromBlockIDIndex(blockID, uint32(i))
		l.store[txIDKey] = &txResult
		l.indexStore[txIndexKey] = &txResult
	}

	return nil
}

// BatchStore inserts a batch of transaction result into a batch.
// This method is not implemented and will always return an error.
func (l *LightTransactionResults) BatchStore(flow.Identifier, []flow.LightTransactionResult, storage.ReaderBatchWriter) error {
	return fmt.Errorf("not implemented")
}

// BatchStoreBadger inserts a batch of transaction result into a storage.
// Deprecated: deprecated as a part of transition from Badger to Pebble. use BatchStore instead.
// This method is not implemented and will always return an error.
func (l *LightTransactionResults) BatchStoreBadger(flow.Identifier, []flow.LightTransactionResult, storage.BatchStorage) error {
	return fmt.Errorf("not implemented")
}
