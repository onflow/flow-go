package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {

	// StoreLight inserts the collection. It does not insert, nor check
	// existence of, the constituent transactions.
	StoreLight(collection *flow.LightCollection) error

	// Store inserts the collection keyed by ID and all constituent
	// transactions.
	Store(collection *flow.Collection) error

	// Remove removes the collection and all constituent transactions.
	Remove(collID flow.Identifier) error

	// LightByID returns collection with the given ID. Only retrieves
	// transaction hashes.
	LightByID(collID flow.Identifier) (*flow.LightCollection, error)

	// ByID returns the collection with the given ID, including all
	// transactions within the collection.
	ByID(collID flow.Identifier) (*flow.Collection, error)

	// StoreLightAndIndexByTransaction inserts the light collection (only
	// transaction IDs) and adds a transaction id index for each of the
	// transactions within the collection (transaction_id->collection_id).
	//
	// NOTE: Currently it is possible in rare circumstances for two collections
	// to be guaranteed which both contain the same transaction (see https://github.com/dapperlabs/flow-go/issues/3556).
	// The second of these will revert upon reaching the execution node, so
	// this doesn't impact the execution state, but it can result in the Access
	// node processing two collections which both contain the same transaction (see https://github.com/dapperlabs/flow-go/issues/5337).
	// To handle this, we skip indexing the affected transaction when inserting
	// the transaction_id->collection_id index when an index for the transaction
	// already exists.
	StoreLightAndIndexByTransaction(collection *flow.LightCollection) error

	// LightByTransactionID returns the collection for the given transaction ID. Only retrieves
	// transaction hashes.
	LightByTransactionID(txID flow.Identifier) (*flow.LightCollection, error)
}
