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

	// LightByID returns a reduced representation of the collection with the given ID.
	// The reduced collection references the constituent transactions by their hashes.
	//
	// Expected errors during normal operation:
	//   - `storage.ErrNotFound` if no light collection was found.
	LightByID(collID flow.Identifier) (*flow.LightCollection, error)

	// ExistByID checks whether a collection with the given ID exists in storage.
	// Returns (true, nil) if it exists,
	// Returns (false, nil) if it does not exist.
	// No errors are expected during normal operation.
	ExistByID(collID flow.Identifier) (bool, error)

	// LightByTransactionID returns a reduced representation of the collection
	// holding the given transaction ID. The reduced collection references the
	// constituent transactions by their hashes.
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
	// This is used by execution node storing collections.
	// No errors are expected during normal operation.
	Store(collection *flow.Collection) (*flow.LightCollection, error)

	// Remove removes the collection and all constituent transactions.
	// No errors are expected during normal operation.
	Remove(collID flow.Identifier) error

	// StoreAndIndexByTransaction stores the collection and indexes it by transaction.
	// This is used by access node storing collections for finalized blocks.
	//
	// deprecated, only used by AN ingestion engine, which is deprecated
	// CAUTION: current approach is NOT BFT and needs to be revised in the future.
	// Honest clusters ensure a transaction can only belong to one collection. However, in rare
	// cases, the collector clusters can exceed byzantine thresholds -- making it possible to
	// produce multiple finalized collections (aka guaranteed collections) containing the same
	// transaction repeatedly.
	// TODO: eventually we need to handle Byzantine clusters
	//
	// No errors are expected during normal operation.
	StoreAndIndexByTransaction(lctx lockctx.Proof, collection *flow.Collection) (*flow.LightCollection, error)

	// BatchStoreAndIndexByTransaction stores a collection and indexes it by transaction ID within a batch.
	//
	// CAUTION:
	//   - The current approach is NOT BFT and needs to be revised in the future. Specifically, we assume that a transaction is only ever
	//     included in a single guaranteed collection (non-BFT shortcut). However, in rare cases, due to sampling anomalies, a cluster can
	//     exceed byzantine threshold (1/3 of cluster stake), making it possible to produce multiple finalized collections containing the
	//     same transaction repeatedly.
	//     TODO: the mature protocol needs to handle Byzantine clusters, which requires generalization (one-to-many mapping).
	//   - At the moment, the information which collection a transaction belongs to should be persisted once and never changed. This is
	//     enforced by the function (logging conflicting membership with the keyword [logging.KeySuspicious], but not overwriting it), for
	//     which reason the caller must hold the [storage.LockInsertAndIndexTxResult] lock.
	//
	// No errors are expected during normal operations
	BatchStoreAndIndexByTransaction(lctx lockctx.Proof, collection *flow.Collection, batch ReaderBatchWriter) (*flow.LightCollection, error)
}
