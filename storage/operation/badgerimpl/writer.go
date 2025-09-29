package badgerimpl

import (
	"fmt"
	"slices"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ReaderBatchWriter is for reading and writing to a storage backend.
// It is useful for performing a related sequence of reads and writes, after which you would like
// to modify some non-database state if the sequence completed successfully (via AddCallback).
// If you are not using AddCallback, avoid using ReaderBatchWriter: use Reader and Writer directly.
// ReaderBatchWriter is not safe for concurrent use.
type ReaderBatchWriter struct {
	globalReader storage.Reader
	batch        *badger.WriteBatch

	// for executing callbacks after the batch has been flushed, such as updating caches
	callbacks *operation.Callbacks

	// values contains the values for this batch.
	// The values map is set using SetScopedValue(key, value) and retrieved using ScopedValue(key).
	// Initialization of the values map is deferred until it is needed, because
	// ReaderBatchWriter is created frequently to update the database, but
	// this values map is used infrequently to save data for batch operations.
	// For example, store.TransactionResults.BatchRemoveByBlockID() saves batch
	// removed block IDs in values map, and retrieves the batch removed block
	// IDs in OnCommitSucceed() callback.  This allows locking just once,
	// instead of locking TransactionResults cache for every removed block ID.
	values map[string]any
}

var _ storage.ReaderBatchWriter = (*ReaderBatchWriter)(nil)
var _ storage.Batch = (*ReaderBatchWriter)(nil)

// GlobalReader returns a database-backed reader which reads the latest committed global database state ("read-committed isolation").
// This reader will not read un-committed writes written to ReaderBatchWriter.Writer until the write batch is committed.
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

// Close releases memory of the batch and no error is returned.
// This can be called as a defer statement immediately after creating Batch
// to reduce risk of unbounded memory consumption.
// No errors are expected during normal operation.
func (b *ReaderBatchWriter) Close() error {
	// BadgerDB v2 docs for WriteBatch.Cancel():
	//
	// "Cancel function must be called if there's a chance that Flush might not get
	// called. If neither Flush or Cancel is called, the transaction oracle would
	// never get a chance to clear out the row commit timestamp map, thus causing an
	// unbounded memory consumption. Typically, you can call Cancel as a defer
	// statement right after NewWriteBatch is called.
	//
	// Note that any committed writes would still go through despite calling Cancel."

	b.batch.Cancel()
	return nil
}

func WithReaderBatchWriter(db *badger.DB, fn func(storage.ReaderBatchWriter) error) error {
	batch := NewReaderBatchWriter(db)
	defer batch.Close() // Release memory

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
		callbacks:    operation.NewCallbacks(),
	}
}

var _ storage.Writer = (*ReaderBatchWriter)(nil)

// Set sets the value for the given key. It overwrites any previous value
// for that key; a DB is not a multi-map.
//
// It is safe to modify the contents of the arguments after Set returns.
// No errors expected during normal operation
func (b *ReaderBatchWriter) Set(key, value []byte) error {
	// BadgerDB v2 docs for WriteBatch.Set() says:
	//
	//   "Set is equivalent of Txn.Set()."
	//
	// BadgerDB v2 docs for Txn.Set() says:
	//
	//   "Set adds a key-value pair to the database.
	//   ...
	//   The current transaction keeps a reference to the key and val byte slice
	//   arguments. Users must not modify key and val until the end of the transaction."

	// Make copies of given key and value because:
	// - ReaderBatchWriter.Set() (this function) promises that it is safe to modify
	//   key and value after Set returns, while
	// - BadgerDB's WriteBatch.Set() said users must not modify key and value
	//   until end of transaction.
	keyCopy := slices.Clone(key)
	valueCopy := slices.Clone(value)

	return b.batch.Set(keyCopy, valueCopy)
}

// Delete deletes the value for the given key. Deletes are blind all will
// succeed even if the given key does not exist.
//
// It is safe to modify the contents of the arguments after Delete returns.
// No errors expected during normal operation
func (b *ReaderBatchWriter) Delete(key []byte) error {
	// BadgerDB v2 docs for WriteBatch.Delete() says:
	//
	//   "Set is equivalent of Txn.Delete."
	//
	// BadgerDB v2 docs for Txn.Set() says:
	//
	//   "Delete deletes a key.
	//   ...
	//   The current transaction keeps a reference to the key byte slice argument.
	//   Users must not modify the key until the end of the transaction."

	// Make copies of given key because:
	// - ReaderBatchWriter.Delete() (this function) promises that it is safe to modify
	//   key after Delete returns, while
	// - BadgerDB's WriteBatch.Delete() says users must not modify key until end of transaction.
	keyCopy := slices.Clone(key)

	return b.batch.Delete(keyCopy)
}

// DeleteByRange removes all keys with a prefix that falls within the
// range [start, end], both inclusive.
// It returns error if endPrefix < startPrefix
// no other errors are expected during normal operation
func (b *ReaderBatchWriter) DeleteByRange(globalReader storage.Reader, startPrefix, endPrefix []byte) error {
	err := operation.IterateKeysByPrefixRange(globalReader, startPrefix, endPrefix, func(key []byte) error {
		err := b.batch.Delete(key)
		if err != nil {
			return fmt.Errorf("could not add key to delete batch (%v): %w", key, err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("could not find keys by range to be deleted: %w", err)
	}
	return nil
}

// SetScopedValue stores the given value by the given key in this batch.
// Stored value can be retrieved by the same key via ScopedValue().
func (b *ReaderBatchWriter) SetScopedValue(key string, value any) {
	// Creation of b.values is deferred until needed, so b.values can be nil here.
	// Deleting element from nil b.values (map[string]any) is no-op.
	// Inserting element to b.values requires initializing b.values first.

	if value == nil {
		delete(b.values, key)
		return
	}
	if b.values == nil {
		b.values = make(map[string]any)
	}
	b.values[key] = value
}

// ScopedValue returns the value associated with this batch for the given key and true if key exists,
// or nil and false if key doesn't exist.
func (b *ReaderBatchWriter) ScopedValue(key string) (any, bool) {
	// Creation of b.values is deferred until needed, so b.values can be nil here.
	// Accessing nil b.values (map[string]any) always returns (nil, false).

	v, exists := b.values[key]
	return v, exists
}
