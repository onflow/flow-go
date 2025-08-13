package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// CollectionsReader represents persistent storage read operations for collections.
type CollectionsReader interface {
	// ByID returns the collection with the given ID, including all
	// transactions within the collection.
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if no light collection was found.
	ByID(collID flow.Identifier) (*flow.Collection, error)

	// LightByID returns collection with the given ID. Only retrieves
	// transaction hashes.
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if no light collection was found.
	LightByID(collID flow.Identifier) (*flow.LightCollection, error)

	// LightByTransactionID returns the collection for the given transaction ID. Only retrieves
	// transaction hashes.
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if no light collection was found.
	LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error)
}

// Collections represents persistent storage for collections.
type Collections interface {
	CollectionsReader

	// Store inserts the collection keyed by ID and all constituent
	// transactions.
	// No errors are expected during normal operation.
	Store(collection *flow.Collection) (flow.LightCollection, error)

	// Remove removes the collection and all constituent transactions.
	// No errors are expected during normal operation.
	Remove(collID flow.Identifier) error

	StoreAndIndexByTransaction(collection *flow.Collection) (flow.LightCollection, error)

	BatchStoreAndIndexByTransaction(collection *flow.Collection, batch ReaderBatchWriter) (flow.LightCollection, error)
}
