package pops

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
	op "github.com/onflow/flow-go/storage/operation"
)

type ReaderBatchWriter struct {
	dbReader
	batch *pebble.Batch

	callbacks op.Callbacks
}

var _ storage.PebbleReaderBatchWriter = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) GlobalReader() storage.Reader {
	return b.dbReader
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
		dbReader: dbReader{db: db},
		batch:    db.NewBatch(),
	}
}

var _ storage.Writer = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) Set(key, value []byte) error {
	return b.batch.Set(key, value, pebble.Sync)
}

func (b *ReaderBatchWriter) Delete(key []byte) error {
	return b.batch.Delete(key, pebble.Sync)
}

// DeleteByRange deletes all keys with the given prefix defined by [startPrefix, endPrefix] (both inclusive).
func (b *ReaderBatchWriter) DeleteByRange(_ storage.Reader, startPrefix, endPrefix []byte) error {
	// DeleteRange takes the prefix range with start (inclusive) and end (exclusive, note: not inclusive).
	// therefore, we need to increment the endPrefix to make it inclusive.
	start, end := storage.StartEndPrefixToLowerUpperBound(startPrefix, endPrefix)
	return b.batch.DeleteRange(start, end, pebble.Sync)
}
