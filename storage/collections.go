package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
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
}
