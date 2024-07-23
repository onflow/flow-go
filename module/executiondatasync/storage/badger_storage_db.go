package storage

import (
	"context"

	ds "github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
)

var _ ExecutionDataStorage = (*BadgerDBWrapper)(nil)

// BadgerDBWrapper wraps the BadgerDB to implement the StorageDB interface.
type BadgerDBWrapper struct {
	ds *badgerds.Datastore
}

func NewBadgerDBWrapper(datastorePath string, options *badgerds.Options) (*BadgerDBWrapper, error) {
	ds, err := badgerds.NewDatastore(datastorePath, options)
	if err != nil {
		return nil, err
	}

	return &BadgerDBWrapper{ds}, nil
}

func (b *BadgerDBWrapper) Datastore() ds.Batching {
	return b.ds
}

func (b *BadgerDBWrapper) Close() error {
	return b.ds.Close()
}

func (b *BadgerDBWrapper) CollectGarbage(ctx context.Context) error {
	return b.ds.CollectGarbage(ctx)
}
