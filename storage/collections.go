package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {

	// Store inserts the collection keyed by fingerprint and all constituent
	// transactions.
	Store(collection *flow.Collection) error

	// Remove removes the collection and all constituent transactions.
	Remove(collID flow.Identifier) error

	// ByID returns the collection with the given fingerprint.
	ByID(collID flow.Identifier) (*flow.Collection, error)
}
