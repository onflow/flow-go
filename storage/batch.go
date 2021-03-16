package storage

import "github.com/dgraph-io/badger/v2"

type Transaction interface {
	Set(key, val []byte) error
}

// BatchStorage
type BatchStorage interface {
	GetWriter() *badger.WriteBatch

	// OnSucceed adds a callback to execute after the batch has
	// been successfully flushed.
	// useful for implement the cache where we will only cache
	// after the batch has been succesfully flushed
	OnSucceed(callback func())

	// Flush will call the badger Batch's Flush method, in
	// addition, it will call the callbacks added by
	// OnSucceed
	Flush() error
}
