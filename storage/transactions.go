package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Transactions represents persistent storage for transactions.
type Transactions interface {

	// Store inserts the transaction, keyed by fingerprint.
	Store(tx *flow.TransactionBody) error

	// ByID returns the transaction for the given fingerprint.
	ByID(txID flow.Identifier) (*flow.TransactionBody, error)

	// CollectionID returns the collection for the given transaction ID
	CollectionID(txID flow.Identifier) (flow.Identifier, error)

	// StoreCollectionID stores the collection ID for the given transaction ID
	StoreCollectionID(txID, collectionID flow.Identifier) error
}
