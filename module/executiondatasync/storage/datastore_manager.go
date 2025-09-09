package storage

import (
	"context"

	ds "github.com/ipfs/go-datastore"

	"github.com/onflow/flow-go/storage"
)

// DatastoreManager defines the interface for managing a datastore.
// It provides methods for accessing the datastore, the underlying database,
// closing the datastore, and performing garbage collection.
type DatastoreManager interface {
	// Datastore provides access to the datastore for performing batched
	// read and write operations.
	Datastore() ds.Batching

	// DB returns the raw database object, allowing for more direct
	// access to the underlying database features and operations.
	DB() storage.DB

	// Close terminates the connection to the datastore and releases
	// any associated resources. This method should be called
	// when finished using the datastore to ensure proper resource cleanup.
	Close() error

	// CollectGarbage initiates garbage collection on the datastore
	// to reclaim unused space and optimize performance.
	CollectGarbage(ctx context.Context) error
}
