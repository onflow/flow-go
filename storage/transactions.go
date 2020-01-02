package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Transactions represents persistent storage for transactions.
type Transactions interface {
	// ByFingerprint returns the transaction for the given fingerprint.
	ByFingerprint(fingerprint flow.Fingerprint) (*flow.Transaction, error)

	// Save inserts the transaction, keyed by fingerprint.
	Insert(tx *flow.Transaction) error

	// Remove removes the transaction with the given hash, if it exists.
	Remove(fingerprint flow.Fingerprint) error
}
