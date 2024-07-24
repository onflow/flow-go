package storage

import (
	"context"

	"github.com/ipfs/go-datastore"
)

// DatastoreManager is an interface that defines the methods for managing
// a datastore used for handling execution data. It provides methods to
// access the underlying datastore, perform garbage collection, and handle
// closing operations. Implementations of this interface are expected to
// wrap around different types of datastore's.
type DatastoreManager interface {
	// Datastore provides access to the datastore for performing batched
	// read and write operations.
	Datastore() datastore.Batching
	// DB returns the raw database object, allowing for more direct
	// access to the underlying database features and operations.
	DB() interface{}
	// Close terminates the connection to the datastore and releases
	// any resources associated with it. This method should be called
	// when you are finished using the datastore to ensure resources
	// are properly cleaned up.
	Close() error
	// CollectGarbage initiates garbage collection on the datastore
	// to reclaim unused space and optimize performance.
	CollectGarbage(ctx context.Context) error
}
