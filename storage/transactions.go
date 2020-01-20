package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Transactions represents persistent storage for transactions.
type Transactions interface {

	// Store inserts the transaction, keyed by fingerprint.
	Store(tx *flow.TransactionBody) error

	// Remove removes the transaction with the given hash, if it exists.
	Remove(txID flow.Identifier) error

	// ByID returns the transaction for the given fingerprint.
	ByID(txID flow.Identifier) (*flow.TransactionBody, error)
}
