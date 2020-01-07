package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {
	// ByFingerprint returns the collection with the given fingerprint.
	ByFingerprint(hash flow.Fingerprint) (*flow.Collection, error)

	// Save inserts the collection keyed by fingerprint and all constituent
	// transactions.
	Save(collection *flow.Collection) error

	// Remove removes the collection and all constituent transactions.
	Remove(hash flow.Fingerprint) error
}
