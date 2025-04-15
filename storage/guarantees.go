package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// Guarantees represents persistent storage for collection guarantees.
type Guarantees interface {

	// Store inserts the collection guarantee.
	Store(guarantee *flow.CollectionGuarantee) error

	Index(collID flow.Identifier, guaranteeID flow.Identifier) func(*transaction.Tx) error

	// ByID returns the flow.CollectionGuarantee by its ID.
	ByID(guaranteeID flow.Identifier) (*flow.CollectionGuarantee, error)

	// ByCollectionID retrieves the collection guarantee by collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error)
}
