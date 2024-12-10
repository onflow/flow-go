package storage

import (
	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
)

type Transaction interface {
	Set(key, val []byte) error
}

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

type PebbleReaderBatchWriter interface {
	ReaderWriter() (pebble.Reader, pebble.Writer)
	IndexedBatch() *pebble.Batch
	AddCallback(func())
}
