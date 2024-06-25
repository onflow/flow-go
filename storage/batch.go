package storage

// TODO: rename to writer
type BatchWriter interface {
	Set(key, val []byte) error
	Delete(key []byte) error
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

type TxOps interface {
	Insert(key, val []byte) error
	Update(key, val []byte) error
	Upsert(key, val []byte) error
	Delete(key []byte) error
	Get(key []byte) ([]byte, error)
}
