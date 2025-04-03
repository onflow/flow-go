package unsynchronized

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

type LightTransactionResults struct {
	// TODO: A mutexes aren't strictly necessary here since, by design, data is written only once
	// before any future reads. However, we're keeping it temporarily during active development
	// for safety and debugging purposes. It will be removed once the implementation is finalized.
	store     map[string]flow.LightTransactionResult
	storeLock sync.RWMutex

	indexStore     map[string]flow.LightTransactionResult
	indexStoreLock sync.RWMutex

	blockStore     map[string][]flow.LightTransactionResult
	blockStoreLock sync.RWMutex
}

func NewLightTransactionResults() *LightTransactionResults {
	return &LightTransactionResults{
		store:          make(map[string]flow.LightTransactionResult),
		storeLock:      sync.RWMutex{},
		indexStore:     make(map[string]flow.LightTransactionResult),
		indexStoreLock: sync.RWMutex{},
		blockStore:     make(map[string][]flow.LightTransactionResult),
		blockStoreLock: sync.RWMutex{},
	}
}

var _ storage.LightTransactionResults = (*LightTransactionResults)(nil)

// ByBlockIDTransactionID returns the transaction result for the given block ID and transaction ID
func (l *LightTransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.LightTransactionResult, error) {
	key := store.KeyFromBlockIDTransactionID(blockID, transactionID)
	l.storeLock.RLock()
	val, ok := l.store[key]
	l.storeLock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

// ByBlockIDTransactionIndex returns the transaction result for the given blockID and transaction index
func (l *LightTransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error) {
	key := store.KeyFromBlockIDIndex(blockID, txIndex)
	l.indexStoreLock.RLock()
	val, ok := l.indexStore[key]
	l.indexStoreLock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

// ByBlockID gets all transaction results for a block, ordered by transaction index
func (l *LightTransactionResults) ByBlockID(id flow.Identifier) ([]flow.LightTransactionResult, error) {
	key := store.KeyFromBlockID(id)
	l.blockStoreLock.RLock()
	val, ok := l.blockStore[key]
	l.blockStoreLock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

// Store inserts a transaction results into a storage
func (l *LightTransactionResults) Store(blockID flow.Identifier, transactionResults []flow.LightTransactionResult) error {
	key := store.KeyFromBlockID(blockID)
	l.blockStoreLock.Lock()
	l.blockStore[key] = transactionResults
	l.blockStoreLock.Unlock()

	for i, txResult := range transactionResults {
		txIDKey := store.KeyFromBlockIDTransactionID(blockID, txResult.TransactionID)
		txIndexKey := store.KeyFromBlockIDIndex(blockID, uint32(i))

		l.storeLock.Lock()
		l.store[txIDKey] = txResult
		l.storeLock.Unlock()

		l.indexStoreLock.Lock()
		l.indexStore[txIndexKey] = txResult
		l.indexStoreLock.Unlock()
	}

	return nil
}

func (l *LightTransactionResults) BatchStore(flow.Identifier, []flow.LightTransactionResult, storage.ReaderBatchWriter) error {
	return fmt.Errorf("not implemented")
}

func (l *LightTransactionResults) BatchStoreBadger(flow.Identifier, []flow.LightTransactionResult, storage.BatchStorage) error {
	return fmt.Errorf("not implemented")
}
