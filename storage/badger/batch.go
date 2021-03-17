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

func (b *Batch) OnSucceed(callback func()) {
	b.callbacks = append(b.callbacks, callback)
}

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
