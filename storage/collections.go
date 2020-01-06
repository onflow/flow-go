package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {
	// ByFingerprint returns the collection with the given fingerprint.
	ByFingerprint(hash flow.Fingerprint) (*flow.Collection, error)

	// TransactionsByFingerprint returns the collection's transactions by the
	// collection fingerprint.
	TransactionsByFingerprint(hash flow.Fingerprint) ([]*flow.Transaction, error)

	// Save inserts the collection, keyed by fingerprint.
	Save(collection *flow.Collection) error

	// Remove removes the collection.
	Remove(hash flow.Fingerprint) error
}
