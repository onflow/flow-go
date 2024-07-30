package storage

import (
	"context"
	"fmt"

	"github.com/cockroachdb/pebble"
	ds "github.com/ipfs/go-datastore"
	pebbleds "github.com/ipfs/go-ds-pebble"

	pstorage "github.com/onflow/flow-go/storage/pebble"
)

var _ DatastoreManager = (*PebbleDatastoreManager)(nil)

// PebbleDatastoreManager wraps the PebbleDB to implement the StorageDB interface.
type PebbleDatastoreManager struct {
	ds *pebbleds.Datastore
	db *pebble.DB
}

// NewPebbleDatastoreManager creates and returns a new instance of PebbleDatastoreManager.
// It initializes the PebbleDB database with the specified path and options.
// If no options are provided, default options are used.
//
// Parameters:
//   - path: The path to the directory where the PebbleDB files will be stored.
//   - options: Configuration options for the PebbleDB database. If nil, default
//     options are applied.
//
// No errors are expected during normal operations.
func NewPebbleDatastoreManager(path string, options *pebble.Options) (*PebbleDatastoreManager, error) {
	if options == nil {
		cache := pebble.NewCache(pstorage.DefaultPebbleCacheSize)
		defer cache.Unref()
		options = pstorage.DefaultPebbleOptions(cache, pebble.DefaultComparer)
	}

	db, err := pebble.Open(path, options)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	ds, err := pebbleds.NewDatastore(path, options, pebbleds.WithPebbleDB(db))
	if err != nil {
		return nil, fmt.Errorf("could not open tracker ds: %w", err)
	}

	return &PebbleDatastoreManager{
		ds: ds,
		db: db,
	}, nil
}

// Datastore provides access to the datastore for performing batched
// read and write operations.
func (p *PebbleDatastoreManager) Datastore() ds.Batching {
	return p.ds
}

// DB returns the raw database object, allowing for more direct
// access to the underlying database features and operations.
func (p *PebbleDatastoreManager) DB() interface{} {
	return p.db
}

// Close terminates the connection to the datastore and releases
// any associated resources. This method should be called
// when finished using the datastore to ensure proper resource cleanup.
func (p *PebbleDatastoreManager) Close() error {
	return p.ds.Close()
}

// CollectGarbage initiates garbage collection on the datastore
// to reclaim unused space and optimize performance.
func (p *PebbleDatastoreManager) CollectGarbage(_ context.Context) error {
	// In PebbleDB, there's no direct equivalent to manual value log garbage collection
	return nil
}
