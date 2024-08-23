package storage

import (
	"io"

	"github.com/dgraph-io/badger/v2"
)

// deprecated
// use Writer instead
type Transaction interface {
	Set(key, val []byte) error
}

// deprecated
// use BadgerReaderBatchWriter instead
// BatchStorage serves as an abstraction over batch storage, adding ability to add ability to add extra
// callbacks which fire after the batch is successfully flushed.
type BatchStorage interface {
	GetWriter() *badger.WriteBatch

	// OnSucceed adds a callback to execute after the batch has
	// been successfully flushed.
	// useful for implementing the cache where we will only cache
	// after the batch has been successfully flushed
	OnSucceed(callback func())

	// Flush will flush the write batch and update the cache.
	Flush() error
}

type Reader interface {
	// Get gets the value for the given key. It returns ErrNotFound if the DB
	// does not contain the key.
	//
	// The caller should not modify the contents of the returned slice, but it is
	// safe to modify the contents of the argument after Get returns. The
	// returned slice will remain valid until the returned Closer is closed. On
	// success, the caller MUST call closer.Close() or a memory leak will occur.
	Get(key []byte) (value []byte, closer io.Closer, err error)
}

// Writer is an interface for batch writing to a storage backend.
type Writer interface {
	// Set sets the value for the given key. It overwrites any previous value
	// for that key; a DB is not a multi-map.
	//
	// It is safe to modify the contents of the arguments after Set returns.
	Set(k, v []byte) error

	// Delete deletes the value for the given key. Deletes are blind all will
	// succeed even if the given key does not exist.
	//
	// It is safe to modify the contents of the arguments after Delete returns.
	Delete(key []byte) error

	// DeleteRange deletes all of the point keys (and values) in the range
	// [start,end) (inclusive on start, exclusive on end). DeleteRange does NOT
	// delete overlapping range keys (eg, keys set via RangeKeySet).
	//
	// It is safe to modify the contents of the arguments after DeleteRange
	// returns.
	DeleteRange(start, end []byte) error
}

// BadgerReaderBatchWriter is an interface for badger-specific reader and writer.
type BadgerReaderBatchWriter interface {
	// ReaderWriter returns the reader and writer for the storage backend.
	// The reader is used to read data from the storage backend, and
	// the writer is used to write data to the storage backend with an atomic batch
	// update.
	// Note:
	// - There is no guarantee on the consistency of the data read,
	// 	 the data read may not reflect the latest data written.
	//   it is the responsibility of the caller to ensure the consistency.
	// - The writer cannot be used concurrently for writing.
	ReaderWriter() (Reader, Writer)

	// BadgerBatch returns the underlying batch object
	// Useful for implementing badger-specific operations
	BadgerWriteBatch() *badger.WriteBatch

	// AddCallback adds a callback to execute after the batch has been flush
	// regardless the batch update is succeeded or failed.
	// The error parameter is the error returned by the batch update.
	AddCallback(func(error))
}

// OnlyBadgerWriter is an adapter to convert a function that takes a Writer
// to a function that takes a BadgerReaderBatchWriter.
func OnlyBadgerWriter(fn func(Writer) error) func(BadgerReaderBatchWriter) error {
	return func(rw BadgerReaderBatchWriter) error {
		_, w := rw.ReaderWriter()
		return fn(w)
	}
}

// OnCommitSucceed adds a callback to execute after the batch has been successfully committed.
func OnCommitSucceed(b BadgerReaderBatchWriter, onSuccessFn func()) {
	b.AddCallback(func(err error) {
		if err == nil {
			onSuccessFn()
		}
	})
}
