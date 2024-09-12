package operation

import (
	"errors"
	"io"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	op "github.com/onflow/flow-go/storage/operation"
)

type ReaderBatchWriter struct {
	db    *pebble.DB
	batch *pebble.Batch

	callbacks op.Callbacks
}

var _ storage.PebbleReaderBatchWriter = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) GlobalReader() storage.Reader {
	return b
}

func (b *ReaderBatchWriter) Writer() storage.Writer {
	return b
}

func (b *ReaderBatchWriter) PebbleWriterBatch() *pebble.Batch {
	return b.batch
}

func (b *ReaderBatchWriter) AddCallback(callback func(error)) {
	b.callbacks.AddCallback(callback)
}

func (b *ReaderBatchWriter) Commit() error {
	err := b.batch.Commit(pebble.Sync)

	b.callbacks.NotifyCallbacks(err)

	return err
}

func WithReaderBatchWriter(db *pebble.DB, fn func(storage.PebbleReaderBatchWriter) error) error {
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

func NewReaderBatchWriter(db *pebble.DB) *ReaderBatchWriter {
	return &ReaderBatchWriter{
		db:    db,
		batch: db.NewBatch(),
	}
}

// ToReader is a helper function to convert a *pebble.DB to a Reader
func ToReader(db *pebble.DB) storage.Reader {
	return NewReaderBatchWriter(db)
}

/* implementing storage.Reader interface for ReaderBatchWriter */

var _ storage.Reader = (*ReaderBatchWriter)(nil)

type noopCloser struct{}

var _ io.Closer = (*noopCloser)(nil)

func (noopCloser) Close() error { return nil }

func (b *ReaderBatchWriter) Get(key []byte) ([]byte, io.Closer, error) {
	value, closer, err := b.db.Get(key)

	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil, storage.ErrNotFound
		}

		// exception while checking for the key
		return nil, nil, irrecoverable.NewExceptionf("could not load data: %w", err)
	}

	return value, closer, nil
}

func (b *ReaderBatchWriter) NewIter(start, end []byte, ops storage.IteratorOption) (storage.Iterator, error) {
	// TODO: end + 1?
	return newPebbleIterator(b.db, start, end, ops)
}

var _ storage.Writer = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) Set(key, value []byte) error {
	return b.batch.Set(key, value, pebble.Sync)
}

func (b *ReaderBatchWriter) Delete(key []byte) error {
	return b.batch.Delete(key, pebble.Sync)
}

func (b *ReaderBatchWriter) DeleteByRange(_ storage.Reader, startPrefix, endPrefix []byte) error {
	return b.batch.DeleteRange(startPrefix, endPrefix, pebble.Sync)
}
