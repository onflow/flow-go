package storage

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {
	// ByHash returns the transaction for the given block.
	ByHash(hash crypto.Hash) (*flow.Collection, error)

	// Insert inserts the transaction, keyed by hash.
	Insert(collection *flow.Collection) error
}
