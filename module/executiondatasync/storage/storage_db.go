package storage

import (
	"context"

	"github.com/ipfs/go-datastore"
)

// TODO: rename
// StorageDB defines the interface for key-value store operations.
type StorageDB interface {
	Datastore() datastore.Batching
	DB() interface{}

	Get(key []byte) (StorageItem, error)
	Set(key, val []byte) error
	Delete(key []byte) error
	Close() error

	Keys(prefix []byte) ([][]byte, error)

	CollectGarbage(ctx context.Context) error

	RetryOnConflict(fn func() error) error
	MaxBatchCount() int64
	MaxBatchSize() int64
	RunValueLogGC(discardRatio float64) error
}

type StorageItem interface {
	ValueCopy(dst []byte) ([]byte, error)
	Key() []byte
}
