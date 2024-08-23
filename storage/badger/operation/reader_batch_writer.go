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

	// callbacks are executed regardless of the success of the batch commit.
	// if any function that is adding writes to the batch fails, the callbacks
	// are also called with the error, in this case the callbacks are executed
	// before the batch is submitted. This is useful for the locks in those functions
	// to be released.
	// callbacks must be non-blocking
	callbacks []func(error)
}

var _ storage.BadgerReaderBatchWriter = (*ReaderBatchWriter)(nil)

// GlobalReader returns a database-backed reader which reads the latest committed global database state ("read-committed isolation").
// This reader will not read writes written to ReaderBatchWriter.Writer until the write batch is committed.
// This reader may observe different values for the same key on subsequent reads.
func (b *ReaderBatchWriter) GlobalReader() storage.Reader {
	return b
}

// Writer returns a writer associated with a batch of writes. The batch is pending until it is committed.
// When we `Write` into the batch, that write operation is added to the pending batch, but not committed.
// The commit operation is atomic w.r.t. the batch; either all writes are applied to the database, or no writes are.
// Note:
// - The writer cannot be used concurrently for writing.
func (b *ReaderBatchWriter) Writer() storage.Writer {
	return b
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
func ToReader(db *badger.DB) storage.Reader {
	return NewReaderBatchWriter(db)
}

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
