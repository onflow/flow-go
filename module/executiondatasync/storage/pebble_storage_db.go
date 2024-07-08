package storage

import (
	"context"
	"fmt"

	"github.com/cockroachdb/pebble"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	pebbleds "github.com/ipfs/go-ds-pebble"
)

var _ StorageDB = (*PebbleDBWrapper)(nil)

// PebbleDBWrapper wraps the PebbleDB to implement the StorageDB interface.
type PebbleDBWrapper struct {
	ds *pebbleds.Datastore
}

func NewPebbleDBWrapper(datastorePath string, options *pebble.Options) (*PebbleDBWrapper, error) {
	db, err := pebbleds.NewDatastore(datastorePath, options)
	if err != nil {
		return nil, fmt.Errorf("could not open tracker ds: %w", err)
	}

	return &PebbleDBWrapper{db}, nil
}

func (p *PebbleDBWrapper) Datastore() ds.Batching {
	return p.ds
}

func (p *PebbleDBWrapper) Keys(prefix []byte) ([][]byte, error) {
	var keys [][]byte
	var q query.Query
	q.Prefix = string(prefix[:])

	result, err := p.ds.Query(context.Background(), q)
	if err != nil {
		return nil, err
	}

	entries, _ := result.Rest()
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		keys = append(keys, []byte(entry.Key))
	}

	return keys, nil
}

func (p *PebbleDBWrapper) CollectGarbage(ctx context.Context) error {
	return nil
}

func (p *PebbleDBWrapper) Get(key []byte) (StorageItem, error) {
	val, err := p.ds.Get(context.Background(), ds.NewKey(string(key)))
	if err != nil {
		return &PebbleItem{}, err
	}
	return &PebbleItem{key: key, val: val}, nil
}

func (p *PebbleDBWrapper) Set(key, val []byte) error {
	return p.ds.Put(context.Background(), ds.NewKey(string(key)), val)
}

func (p *PebbleDBWrapper) Delete(key []byte) error {
	return p.ds.Delete(context.Background(), ds.NewKey(string(key)))
}

func (p *PebbleDBWrapper) Close() error {
	return p.ds.Close()
}

func (p *PebbleDBWrapper) RetryOnConflict(_ func() error) error {
	return nil
}

// TODO: implement
func (p *PebbleDBWrapper) MaxBatchCount() int64 {
	return 0
}

// TODO: implement
func (p *PebbleDBWrapper) MaxBatchSize() int64 {
	return 0
}

func (p *PebbleDBWrapper) RunValueLogGC(_ float64) error {
	// PebbleDB (go-ds-pebble) does not have a direct equivalent to Badger's value log GC.
	return nil
}

var _ StorageItem = (*PebbleItem)(nil)

type PebbleItem struct {
	key []byte
	val []byte
}

func (i *PebbleItem) ValueCopy(dst []byte) ([]byte, error) {
	return append(dst, i.val...), nil
}

func (i *PebbleItem) Key() []byte {
	return i.key
}
