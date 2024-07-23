package storage

import (
	"context"
	"fmt"

	"github.com/cockroachdb/pebble"
	ds "github.com/ipfs/go-datastore"
	pebbleds "github.com/ipfs/go-ds-pebble"

	pstorage "github.com/onflow/flow-go/storage/pebble"
)

var _ ExecutionDataStorage = (*PebbleDBWrapper)(nil)

// PebbleDBWrapper wraps the PebbleDB to implement the StorageDB interface.
type PebbleDBWrapper struct {
	ds *pebbleds.Datastore
	db *pebble.DB
}

func NewPebbleDBWrapper(dbPath string, options *pebble.Options) (*PebbleDBWrapper, error) {
	if options == nil {
		cache := pebble.NewCache(pstorage.DefaultPebbleCacheSize)
		defer cache.Unref()
		options = pstorage.DefaultPebbleOptions(cache, pebble.DefaultComparer)
	}

	db, err := pebble.Open(dbPath, options)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	ds, err := pebbleds.NewDatastore(dbPath, options, pebbleds.WithPebbleDB(db))
	if err != nil {
		return nil, fmt.Errorf("could not open tracker ds: %w", err)
	}

	return &PebbleDBWrapper{
		ds: ds,
		db: db,
	}, nil
}

func (p *PebbleDBWrapper) Datastore() ds.Batching {
	return p.ds
}

func (p *PebbleDBWrapper) DB() interface{} {
	return p.db
}

func (p *PebbleDBWrapper) Close() error {
	return p.ds.Close()
}

func (p *PebbleDBWrapper) CollectGarbage(_ context.Context) error {
	return nil
}
