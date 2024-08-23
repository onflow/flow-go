package operation

import (
	"errors"
	"io"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type ReaderBatchWriter struct {
	db    *badger.DB
	batch *badger.WriteBatch

	addingCallback sync.Mutex // protect callbacks
	callbacks      []func(error)
}

var _ storage.BadgerReaderBatchWriter = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) ReaderWriter() (storage.Reader, storage.Writer) {
	// reusing the same underlying object, but expose with different interfaces
	return b, b
}

func (b *ReaderBatchWriter) BadgerWriteBatch() *badger.WriteBatch {
	return b.batch
}

func (b *ReaderBatchWriter) AddCallback(callback func(error)) {
	b.addingCallback.Lock()
	defer b.addingCallback.Unlock()

	b.callbacks = append(b.callbacks, callback)
}

func (b *ReaderBatchWriter) Commit() error {
	err := b.batch.Flush()

	b.notifyCallbacks(err)

	return err
}

func (b *ReaderBatchWriter) notifyCallbacks(err error) {
	b.addingCallback.Lock()
	defer b.addingCallback.Unlock()

	for _, callback := range b.callbacks {
		callback(err)
	}
}

func WithReaderBatchWriter(db *badger.DB, fn func(storage.BadgerReaderBatchWriter) error) error {
	batch := NewReaderBatchWriter(db)

	err := fn(batch)
	if err != nil {
		// fn might use lock to ensure concurrent safety while reading and writing data
		// and the lock is usually released by a callback.
		// in other words, fn might hold a lock to be released by a callback,
		// we need to notify the callback for the locks to be released before
		// returning the error.
		batch.notifyCallbacks(err)
		return err
	}

	return batch.Commit()
}

func NewReaderBatchWriter(db *badger.DB) *ReaderBatchWriter {
	return &ReaderBatchWriter{
		db:    db,
		batch: db.NewWriteBatch(),
	}
}

// ToReader is a helper function to convert a *badger.DB to a Reader
var ToReader = NewReaderBatchWriter

var _ storage.Reader = (*ReaderBatchWriter)(nil)

type noopCloser struct{}

var _ io.Closer = (*noopCloser)(nil)

func (noopCloser) Close() error { return nil }

func (b *ReaderBatchWriter) Get(key []byte) ([]byte, io.Closer, error) {
	tx := b.db.NewTransaction(false)
	defer tx.Discard()

	item, err := tx.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, nil, storage.ErrNotFound
		}
		return nil, nil, irrecoverable.NewExceptionf("could not load data: %w", err)
	}

	var value []byte
	err = item.Value(func(val []byte) error {
		value = append([]byte{}, val...)
		return nil
	})
	if err != nil {
		return nil, nil, irrecoverable.NewExceptionf("could not load value: %w", err)
	}

	return value, noopCloser{}, nil
}

var _ storage.Writer = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) Set(key, value []byte) error {
	return b.batch.Set(key, value)
}

func (b *ReaderBatchWriter) Delete(key []byte) error {
	return b.batch.Delete(key)
}
