package pebble

import (
	"sync"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

type Batch struct {
	writer *pebble.Batch

	lock      sync.RWMutex
	callbacks []func()
}

var _ storage.BatchStorage = (*Batch)(nil)

func NewBatch(db *pebble.DB) *Batch {
	batch := db.NewBatch()
	return &Batch{
		writer:    batch,
		callbacks: make([]func(), 0),
	}
}

func (b *Batch) GetWriter() storage.Transaction {
	return b.writer
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
// any error are exceptions
func (b *Batch) Flush() error {
	err := b.writer.Commit(nil)
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

func (b *Batch) Close() error {
	return b.writer.Close()
}
