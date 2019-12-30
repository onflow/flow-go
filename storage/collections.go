package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {
	// ByFingerprint returns the collection for the given fingerprint.
	ByFingerprint(fingerprint flow.Fingerprint) (*flow.Collection, error)

	// Insert inserts the collection, keyed by fingerprint.
	Insert(tx *flow.Collection) error

	// Remove removes the collection with the given hash, if it exists.
	Remove(fingerprint flow.Fingerprint) error
}
