package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Guarantees represents persistent storage for collection guarantees.
type Guarantees interface {

	// Store inserts the collection guarantee.
	Store(guarantee *flow.CollectionGuarantee) error

	// ByID retrieves the collection guarantee by collection ID.
	ByID(collID flow.Identifier) (*flow.CollectionGuarantee, error)

	// ByBlockID returns guarantees from a block.
	ByBlockID(blockID flow.Identifier) ([]*flow.CollectionGuarantee, error)
}
