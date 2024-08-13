package operation

import (
	"errors"

	"github.com/cockroachdb/pebble"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// ReaderBatchWriter is a struct that manages batch operations in a Pebble database.
// It allows for combining multiple database writes into a single batch and provides
// a way to execute callbacks upon batch commit.
type ReaderBatchWriter struct {
	db        *pebble.DB
	batch     *pebble.Batch
	callbacks []func()
}

var _ storage.PebbleReaderBatchWriter = (*ReaderBatchWriter)(nil)

// ReaderWriter returns the Pebble reader and writer.
// This method provides access to the database and the batch for read-write operations.
func (b *ReaderBatchWriter) ReaderWriter() (pebble.Reader, pebble.Writer) {
	return b.db, b.batch
}

// IndexedBatch returns the current batch for indexed write operations.
func (b *ReaderBatchWriter) IndexedBatch() *pebble.Batch {
	return b.batch
}

// Commit commits the current batch to the database.
// This method finalizes the batch operations and persists them to the database.
func (b *ReaderBatchWriter) Commit() error {
	return b.batch.Commit(nil)
}

// AddCallback adds a callback function to be executed after the batch is committed.
// The callbacks are executed in the order they are added.
func (b *ReaderBatchWriter) AddCallback(callback func()) {
	b.callbacks = append(b.callbacks, callback)
}

// NewPebbleReaderBatchWriter creates a new ReaderBatchWriter for the given Pebble database.
// This function initializes the ReaderBatchWriter with a new indexed batch.
func NewPebbleReaderBatchWriter(db *pebble.DB) *ReaderBatchWriter {
	return &ReaderBatchWriter{
		db:    db,
		batch: db.NewIndexedBatch(),
	}
}

// WithReaderBatchWriter executes a function with a ReaderBatchWriter and commits the batch.
// This function ensures that the batch is committed and callbacks are executed.
func WithReaderBatchWriter(db *pebble.DB, fn func(storage.PebbleReaderBatchWriter) error) error {
	batch := NewPebbleReaderBatchWriter(db)
	err := fn(batch)
	if err != nil {
		return err
	}
	err = batch.Commit()
	if err != nil {
		return err
	}

	for _, callback := range batch.callbacks {
		callback()
	}

	return nil
}

// insert returns a function that inserts a key-value pair into the Pebble database.
func insert(key []byte, val interface{}) func(pebble.Writer) error {
	return func(w pebble.Writer) error {
		value, err := msgpack.Marshal(val)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to encode value: %w", err)
		}

		err = w.Set(key, value, nil)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to store data: %w", err)
		}

		return nil
	}
}

// retrieve returns a function that retrieves a value from the Pebble database by key.
func retrieve(key []byte, sc interface{}) func(r pebble.Reader) error {
	return func(r pebble.Reader) error {
		val, closer, err := r.Get(key)
		if err != nil {
			return convertNotFoundError(err)
		}
		defer closer.Close()

		err = msgpack.Unmarshal(val, sc)
		if err != nil {
			return irrecoverable.NewExceptionf("failed to decode value: %w", err)
		}
		return nil
	}
}

// exists returns a function that checks whether a key exists in the Pebble database.
// It sets the provided boolean pointer to true if the key exists, or false otherwise.
func exists(key []byte, keyExists *bool) func(r pebble.Reader) error {
	return func(r pebble.Reader) error {
		_, closer, err := r.Get(key)
		if err != nil {
			if errors.Is(err, pebble.ErrNotFound) {
				*keyExists = false
				return nil
			}

			// exception while checking for the key
			return irrecoverable.NewExceptionf("could not load data: %w", err)
		}
		*keyExists = true
		defer closer.Close()
		return nil
	}
}

// remove removes the entity with the given key, if it exists. If it doesn't
// exist, this is a no-op.
// Error returns:
// * generic error in case of unexpected database error
func remove(key []byte) func(pebble.Writer) error {
	return func(w pebble.Writer) error {
		err := w.Delete(key, nil)
		if err != nil {
			return irrecoverable.NewExceptionf("could not delete item: %w", err)
		}
		return nil
	}
}

func convertNotFoundError(err error) error {
	if errors.Is(err, pebble.ErrNotFound) {
		return storage.ErrNotFound
	}
	return err
}
