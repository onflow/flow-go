package storage

// Transaction is an interface representing a database transaction. It is used
// to prevent this package (storage *interfaces*) from being explicitly dependent
// on Badger, the particular implementation for Flow's database.
// The Badger-dependent implementations
type Transaction interface {
	Set(key, val []byte) error
	Delete(key, val []byte) error
}

// TransactionContext ...
type TransactionContext[tx Transaction] interface {
	GetTx() tx
	OnSucceed(cb func())
}

// WriteBatch is an interface representing a batch of database writes. It is used
// to prevent this package (storage *interfaces*) from being explicitly dependent
// on Badger, the particular implementation for Flow's database.
type WriteBatch interface {
	Set(key, val []byte) error
	Delete(key []byte) error
}

// WriteBatchContext serves as an abstraction over batch storage, adding ability to add ability to add extra
// callbacks which fire after the batch is successfully flushed.
type WriteBatchContext[w WriteBatch] interface {
	GetWriter() w

	// OnSucceed adds a callback to execute after the batch has
	// been successfully flushed.
	// useful for implementing the cache where we will only cache
	// after the batch has been successfully flushed
	OnSucceed(callback func())

	// Flush will flush the write batch and update the cache.
	Flush() error
}
