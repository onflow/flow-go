package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// TransactionsReader represents persistent storage read operations for transactions.
type TransactionsReader interface {
	// ByID returns the transaction for the given fingerprint.
	ByID(txID flow.Identifier) (*flow.TransactionBody, error)
}

// Transactions represents persistent storage for transactions.
type Transactions interface {
	TransactionsReader

	// Store inserts the transaction, keyed by fingerprint. Duplicate transaction insertion is ignored
	Store(tx *flow.TransactionBody) error
}
