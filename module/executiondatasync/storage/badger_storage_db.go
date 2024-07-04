package storage

import (
	"errors"

	"github.com/dgraph-io/badger/v2"
	ds "github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
)

var _ StorageDB = (*BadgerDBWrapper)(nil)

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

func (b *BadgerDBWrapper) Keys(prefix []byte) ([][]byte, error) {
	var keys [][]byte

	err := b.ds.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         prefix,
		})
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			keys = append(keys, it.Item().Key())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (b *BadgerDBWrapper) Get(key []byte) (StorageItem, error) {
	var item StorageItem
	err := b.ds.DB.View(func(txn *badger.Txn) error {
		dbItem, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := dbItem.ValueCopy(nil)
		if err != nil {
			return err
		}
		item = StorageItem{key: key, val: val}
		return nil
	})
	return item, err
}

func (b *BadgerDBWrapper) Set(key, val []byte) error {
	return b.ds.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

func (b *BadgerDBWrapper) Delete(key []byte) error {
	return b.ds.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (b *BadgerDBWrapper) Close() error {
	return b.ds.Close()
}

func (b *BadgerDBWrapper) RetryOnConflict(fn func() error) error {
	for {
		err := fn()
		if errors.Is(err, badger.ErrConflict) {
			continue
		}
		return err
	}
}

func (b *BadgerDBWrapper) MaxBatchCount() int64 {
	return b.ds.DB.MaxBatchCount()
}

func (b *BadgerDBWrapper) MaxBatchSize() int64 {
	return b.ds.DB.MaxBatchSize()
}

func (b *BadgerDBWrapper) RunValueLogGC(discardRatio float64) error {
	return b.ds.DB.RunValueLogGC(discardRatio)
}
