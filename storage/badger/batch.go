package badger

import (
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
)

type Batch struct {
	db     *badger.DB
	writer *badger.WriteBatch

	lock      sync.RWMutex
	callbacks []func()
}

var _ storage.BatchStorage = (*Batch)(nil)

func NewBatch(db *badger.DB) *Batch {
	batch := db.NewWriteBatch()
	return &Batch{
		db:        db,
		writer:    batch,
		callbacks: make([]func(), 0),
	}
}

func (b *Batch) GetWriter() storage.Transaction {
	return b.writer
}

type reader struct {
	db *badger.DB
}

func (r *reader) Get(key []byte) ([]byte, error) {
	var val []byte
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (b *Batch) GetReader() storage.Reader {
	return &reader{db: b.db}
}

// OnSucceed adds a callback to execute after the batch has
// been successfully flushed.
// useful for implementing the cache where we will only cache
// after the batch has been successfully flushed
func (b *Batch) OnSucceed(callback func()) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.callbacks = append(b.callbacks, callback)
}

// Flush will call the badger Batch's Flush method, in
// addition, it will call the callbacks added by
// OnSucceed
func (b *Batch) Flush() error {
	err := b.writer.Flush()
	if err != nil {
		return err
	}

	b.lock.RLock()
	defer b.lock.RUnlock()
	for _, callback := range b.callbacks {
		callback()
	}
	return nil
}
