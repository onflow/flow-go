package storage

import (
	"github.com/dgraph-io/badger/v4"
)

// Deprecated: Transaction is being deprecated as part of the transition from Badger to Pebble.
// Use Writer instead of Transaction for all new code.
type Transaction interface {
	Set(key, val []byte) error
}

// BatchStorage serves as an abstraction over batch storage, adding ability to add ability to add extra
// callbacks which fire after the batch is successfully flushed.
// Deprecated: BatchStorage is being deprecated as part of the transition from Badger to Pebble.
// Use ReaderBatchWriter instead of BatchStorage for all new code.
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
