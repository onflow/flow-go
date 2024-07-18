package storage

import (
	"context"

	"github.com/ipfs/go-datastore"
)

// TODO: rename
// StorageDB defines the interface for key-value store operations.
type StorageDB interface {
	Datastore() datastore.Batching

	Get(key []byte) (StorageItem, error)
	Set(key, val []byte) error
	Delete(key []byte) error
	Close() error

	Keys(prefix []byte) ([][]byte, error)

	CollectGarbage(ctx context.Context) error
}

type StorageItem interface {
	ValueCopy(dst []byte) ([]byte, error)
	Key() []byte
}
