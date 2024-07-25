package storage

import (
	"context"

	ds "github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger2"
)

var _ DatastoreManager = (*BadgerDatastoreManager)(nil)

// BadgerDatastoreManager wraps the Badger datastore to implement the
// DatastoreManager interface. It provides access to a Badger datastore
// instance and implements the required methods for managing it.
type BadgerDatastoreManager struct {
	ds *badgerds.Datastore
}

// NewBadgerDatastoreManager creates a new instance of BadgerDatastoreManager
// with the specified datastore path and options.
//
// Parameters:
// - path: The path where the datastore files will be stored.
// - options: Configuration options for the Badger datastore.
//
// No errors are expected during normal operations.
func NewBadgerDatastoreManager(path string, options *badgerds.Options) (*BadgerDatastoreManager, error) {
	ds, err := badgerds.NewDatastore(path, options)
	if err != nil {
		return nil, err
	}

	return &BadgerDatastoreManager{ds}, nil
}

// Datastore provides access to the datastore for performing batched
// read and write operations.
func (b *BadgerDatastoreManager) Datastore() ds.Batching {
	return b.ds
}

// DB returns the raw database object, allowing for more direct
// access to the underlying database features and operations.
func (b *BadgerDatastoreManager) DB() interface{} {
	return b.ds.DB
}

// Close terminates the connection to the datastore and releases
// any associated resources. This method should be called
// when finished using the datastore to ensure proper resource cleanup.
func (b *BadgerDatastoreManager) Close() error {
	return b.ds.Close()
}

// CollectGarbage initiates garbage collection on the Badger datastore
// to reclaim unused space and optimize performance.
func (b *BadgerDatastoreManager) CollectGarbage(ctx context.Context) error {
	return b.ds.CollectGarbage(ctx)
}
