package badgerimpl

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	op "github.com/onflow/flow-go/storage/operation"
)

type ReaderBatchWriter struct {
	globalReader storage.Reader
	batch        *badger.WriteBatch

	callbacks op.Callbacks
}

var _ storage.ReaderBatchWriter = (*ReaderBatchWriter)(nil)

// GlobalReader returns a database-backed reader which reads the latest committed global database state ("read-committed isolation").
// This reader will not read writes written to ReaderBatchWriter.Writer until the write batch is committed.
// This reader may observe different values for the same key on subsequent reads.
func (b *ReaderBatchWriter) GlobalReader() storage.Reader {
	return b.globalReader
}

// Writer returns a writer associated with a batch of writes. The batch is pending until it is committed.
// When we `Write` into the batch, that write operation is added to the pending batch, but not committed.
// The commit operation is atomic w.r.t. the batch; either all writes are applied to the database, or no writes are.
// Note:
// - The writer cannot be used concurrently for writing.
func (b *ReaderBatchWriter) Writer() storage.Writer {
	return b
}

// BadgerWriteBatch returns the badger write batch
func (b *ReaderBatchWriter) BadgerWriteBatch() *badger.WriteBatch {
	return b.batch
}

// AddCallback adds a callback to execute after the batch has been flush
// regardless the batch update is succeeded or failed.
// The error parameter is the error returned by the batch update.
func (b *ReaderBatchWriter) AddCallback(callback func(error)) {
	b.callbacks.AddCallback(callback)
}

// Commit flushes the batch to the database.
// No errors expected during normal operation
func (b *ReaderBatchWriter) Commit() error {
	err := b.batch.Flush()

	b.callbacks.NotifyCallbacks(err)

	return err
}

func WithReaderBatchWriter(db *badger.DB, fn func(storage.ReaderBatchWriter) error) error {
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
		globalReader: ToReader(db),
		batch:        db.NewWriteBatch(),
	}
}

var _ storage.Writer = (*ReaderBatchWriter)(nil)

// Set sets the value for the given key. It overwrites any previous value
// for that key; a DB is not a multi-map.
//
// It is safe to modify the contents of the arguments after Set returns.
// No errors expected during normal operation
func (b *ReaderBatchWriter) Set(key, value []byte) error {
	return b.batch.Set(key, value)
}

// Delete deletes the value for the given key. Deletes are blind all will
// succeed even if the given key does not exist.
//
// It is safe to modify the contents of the arguments after Delete returns.
// No errors expected during normal operation
func (b *ReaderBatchWriter) Delete(key []byte) error {
	return b.batch.Delete(key)
}

// DeleteByRange removes all keys with a prefix that falls within the
// range [start, end], both inclusive.
// No errors expected during normal operation
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
