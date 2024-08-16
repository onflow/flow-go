package storage

import "github.com/cockroachdb/pebble"

// TODO: rename to writer
type BatchWriter interface {
	Set(key, val []byte) error
	Delete(key []byte) error
	DeleteRange(start, end []byte) error
}

type Reader interface {
	Get(key []byte) ([]byte, error)
}

// BatchStorage serves as an abstraction over batch storage, adding ability to add ability to add extra
// callbacks which fire after the batch is successfully flushed.
type BatchStorage interface {
	GetWriter() BatchWriter
	GetReader() Reader

	// OnSucceed adds a callback to execute after the batch has
	// been successfully flushed.
	// useful for implementing the cache where we will only cache
	// after the batch has been successfully flushed
	OnSucceed(callback func())

	// Flush will flush the write batch and update the cache.
	Flush() error
}

type PebbleReaderBatchWriter interface {
	ReaderWriter() (pebble.Reader, pebble.Writer)
	IndexedBatch() *pebble.Batch
	AddCallback(func(error))
}

func OnlyWriter(fn func(pebble.Writer) error) func(PebbleReaderBatchWriter) error {
	return func(rw PebbleReaderBatchWriter) error {
		_, w := rw.ReaderWriter()
		return fn(w)
	}
}
