package storage

import (
	"github.com/jordanschalm/lockctx"

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
	// This is used by execution node storing collections
	// No errors are expected during normal operation.
	// No errors are expected during normal operation.
	Store(collection *flow.Collection) (flow.LightCollection, error)

	// Remove removes the collection and all constituent transactions.
	// No errors are expected during normal operation.
	Remove(collID flow.Identifier) error

	// StoreAndIndexByTransaction stores the collection and indexes it by transaction.
	// This is used by access node storing collections for finalized blocks
	// No errors are expected during normal operation.
	StoreAndIndexByTransaction(lctx lockctx.Proof, collection *flow.Collection) (flow.LightCollection, error)

	// BatchStoreAndIndexByTransaction stores the collection and indexes it by transaction within a batch.
	// This is used by access node storing collections for finalized blocks
	BatchStoreAndIndexByTransaction(lctx lockctx.Proof, collection *flow.Collection, batch ReaderBatchWriter) (flow.LightCollection, error)
}
