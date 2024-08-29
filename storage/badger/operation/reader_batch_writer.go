package operation

import (
	"errors"
	"io"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	op "github.com/onflow/flow-go/storage/operation"
)

type ReaderBatchWriter struct {
	db    *badger.DB
	batch *badger.WriteBatch

	callbacks op.Callbacks
}

var _ storage.BadgerReaderBatchWriter = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) GlobalReader() storage.Reader {
	return b
}

func (b *ReaderBatchWriter) Writer() storage.Writer {
	return b
}

func (b *ReaderBatchWriter) BadgerWriteBatch() *badger.WriteBatch {
	return b.batch
}

func (b *ReaderBatchWriter) AddCallback(callback func(error)) {
	b.callbacks.AddCallback(callback)
}

func (b *ReaderBatchWriter) Commit() error {
	err := b.batch.Flush()

	b.callbacks.NotifyCallbacks(err)

	return err
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
		batch.callbacks.NotifyCallbacks(err)
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
func ToReader(db *badger.DB) storage.Reader {
	return NewReaderBatchWriter(db)
}

/* implementing storage.Reader interface for ReaderBatchWriter */

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

func (b *ReaderBatchWriter) NewIter(start, end []byte, ops storage.IteratorOption) (storage.Iterator, error) {
	return newBadgerIterator(b.db, start, end, ops), nil
}

var _ storage.Writer = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) Set(key, value []byte) error {
	return b.batch.Set(key, value)
}

func (b *ReaderBatchWriter) Delete(key []byte) error {
	return b.batch.Delete(key)
}
