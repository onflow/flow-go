package bops

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	op "github.com/onflow/flow-go/storage/operation"
)

type ReaderBatchWriter struct {
	dbReader
	batch *badger.WriteBatch

	callbacks op.Callbacks
}

var _ storage.BadgerReaderBatchWriter = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) GlobalReader() storage.Reader {
	return b.dbReader
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
		dbReader: dbReader{db: db},
		batch:    db.NewWriteBatch(),
	}
}

// ToReader is a helper function to convert a *badger.DB to a Reader
func ToReader(db *badger.DB) storage.Reader {
	return NewReaderBatchWriter(db)
}

var _ storage.Writer = (*ReaderBatchWriter)(nil)

func (b *ReaderBatchWriter) Set(key, value []byte) error {
	return b.batch.Set(key, value)
}

func (b *ReaderBatchWriter) Delete(key []byte) error {
	return b.batch.Delete(key)
}

func (b *ReaderBatchWriter) DeleteByRange(globalReader storage.Reader, startPrefix, endPrefix []byte) error {
	err := operation.IterateKeysInPrefixRange(startPrefix, endPrefix, func(key []byte) error {
		err := b.batch.Delete(key)
		if err != nil {
			return fmt.Errorf("could not add key to delete batch (%v): %w", key, err)
		}
		return nil
	})(globalReader)

	if err != nil {
		return fmt.Errorf("could not find keys by range to be deleted: %w", err)
	}
	return nil
}
