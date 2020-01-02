package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {
	// ByFingerprint returns the collection with the given fingerprint.
	ByFingerprint(hash flow.Fingerprint) (*flow.GuaranteedCollection, error)

	// ByFingerprintWithTransactions returns the collection's transactions by the
	// collection fingerprint.
	ByFingerprintWithTransactions(hash flow.Fingerprint) ([]*flow.Transaction, error)

	// Insert inserts the collection, keyed by hash.
	Insert(collection *flow.GuaranteedCollection) error

	// Remove removes the collection.
	Remove(hash flow.Fingerprint) error
}
