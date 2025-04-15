package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Transactions represents persistent storage for transactions.
type Transactions interface {

	// Store inserts the transaction, keyed by fingerprint. Duplicate transaction insertion is ignored
	Store(tx *flow.TransactionBody) error

	// StoreByID inserts the transaction keyed by the given fingerprint txID.
	// Duplicate transaction insertion is ignored.
	// StoreByID should be used instead of Store() when transaction fingerprint is available.
	StoreByID(txID flow.Identifier, tx *flow.TransactionBody) error

	// ByID returns the transaction for the given fingerprint.
	ByID(txID flow.Identifier) (*flow.TransactionBody, error)
}
