package unsynchronized

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/store"
)

type LightTransactionResults struct {
	//TODO: we don't need a mutex here as we have a guarantee by design
	// that we write data only once and it happens before the future reads.
	// We decided to leave a mutex for some time during active development.
	// It'll be removed later on.
	lock       sync.RWMutex
	store      map[string]flow.LightTransactionResult
	indexStore map[string]flow.LightTransactionResult
	blockStore map[string][]flow.LightTransactionResult
}

func NewLightTransactionResults() *LightTransactionResults {
	return &LightTransactionResults{
		store:      make(map[string]flow.LightTransactionResult),
		indexStore: make(map[string]flow.LightTransactionResult),
		blockStore: make(map[string][]flow.LightTransactionResult),
	}
}

var _ storage.LightTransactionResultsReader = (*LightTransactionResults)(nil)

func (l *LightTransactionResults) ByBlockIDTransactionID(blockID flow.Identifier, transactionID flow.Identifier) (*flow.LightTransactionResult, error) {
	key := store.KeyFromBlockIDTransactionID(blockID, transactionID)

	l.lock.RLock()
	val, ok := l.store[key]
	l.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

func (l *LightTransactionResults) ByBlockIDTransactionIndex(blockID flow.Identifier, txIndex uint32) (*flow.LightTransactionResult, error) {
	key := store.KeyFromBlockIDIndex(blockID, txIndex)

	l.lock.RLock()
	val, ok := l.indexStore[key]
	l.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return &val, nil
}

func (l *LightTransactionResults) ByBlockID(id flow.Identifier) ([]flow.LightTransactionResult, error) {
	key := store.KeyFromBlockID(id)

	l.lock.RLock()
	val, ok := l.blockStore[key]
	l.lock.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	return val, nil
}

func (l *LightTransactionResults) Store(blockID flow.Identifier, transactionResults []flow.LightTransactionResult) error {
	key := store.KeyFromBlockID(blockID)
	l.lock.Lock()
	l.blockStore[key] = transactionResults
	l.lock.Unlock()

	for i, txResult := range transactionResults {
		txIDKey := store.KeyFromBlockIDTransactionID(blockID, txResult.TransactionID)
		txIndexKey := store.KeyFromBlockIDIndex(blockID, uint32(i))

		l.lock.Lock()
		l.store[txIDKey] = txResult
		l.indexStore[txIndexKey] = txResult
		l.lock.Unlock()
	}

	return nil
}

func (l *LightTransactionResults) AddToBatch(batch storage.ReaderBatchWriter) error {
	writer := batch.Writer()

	for block, results := range l.blockStore {
		decodedBlock, err := store.KeyToBlockID(block)
		if err != nil {
			return fmt.Errorf("could not decode block: %w", err)
		}

		for _, txResult := range results {
			err = operation.BatchInsertLightTransactionResult(writer, decodedBlock, &txResult)
			if err != nil {
				return fmt.Errorf("could not persist light transaction result: %w", err)
			}
		}
	}

	return nil
}
