package storage

import (
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Transactions represents persistent storage for transactions.
type Transactions interface {
	// ByFingerprint returns the transaction for the given fingerprint.
	ByFingerprint(fingerprint model.Fingerprint) (*flow.Transaction, error)

	// Insert inserts the transaction, keyed by fingerprint.
	Insert(tx *flow.Transaction) error
}
