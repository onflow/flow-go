package badger

import (
	"github.com/dgraph-io/badger/v2"
)

type Batch struct {
	writer    *badger.WriteBatch
	callbacks []func()
}

func NewBatch(db *badger.DB) *Batch {
	batch := db.NewWriteBatch()
	return &Batch{
		writer:    batch,
		callbacks: make([]func(), 0),
	}
}

func (b *Batch) GetWriter() *badger.WriteBatch {
	return b.writer
}

// OnSucceed adds a callback to execute after the batch has
// been successfully flushed.
// useful for implementing the cache where we will only cache
// after the batch has been successfully flushed
func (b *Batch) OnSucceed(callback func()) {
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

	for _, callback := range b.callbacks {
		callback()
	}
	return nil
}
