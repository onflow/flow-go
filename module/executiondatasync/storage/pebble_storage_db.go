package storage

import (
	"context"
	"fmt"

	"github.com/cockroachdb/pebble"
	ds "github.com/ipfs/go-datastore"
	pebbleds "github.com/ipfs/go-ds-pebble"

	pstorage "github.com/onflow/flow-go/storage/pebble"
)

var _ StorageDB = (*PebbleDBWrapper)(nil)

// PebbleDBWrapper wraps the PebbleDB to implement the StorageDB interface.
type PebbleDBWrapper struct {
	ds *pebbleds.Datastore
}

func NewPebbleDBWrapper(dbPath string, options *pebble.Options) (*PebbleDBWrapper, error) {
	if options == nil {
		cache := pebble.NewCache(pstorage.DefaultPebbleCacheSize)
		defer cache.Unref()
		options = pstorage.DefaultPebbleOptions(cache, pebble.DefaultComparer)
	}

	ds, err := pebbleds.NewDatastore(dbPath, options)
	if err != nil {
		return nil, fmt.Errorf("could not open tracker ds: %w", err)
	}

	return &PebbleDBWrapper{
		ds: ds,
	}, nil
}

func (p *PebbleDBWrapper) Datastore() ds.Batching {
	return p.ds
}

func (p *PebbleDBWrapper) Close() error {
	return p.ds.Close()
}

func (p *PebbleDBWrapper) CollectGarbage(_ context.Context) error {
	return nil
}
