package storage

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Transactions represents persistent storage for transactions.
type Transactions interface {
	// ByHash returns the transaction for the given block.
	ByHash(hash crypto.Hash) (*flow.Transaction, error)

	// Insert inserts the transaction, keyed by hash.
	Insert(tx *flow.Transaction) error
}
