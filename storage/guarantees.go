package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Guarantees represents persistent storage for collection guarantees.
type Guarantees interface {

	// Store inserts the collection guarantee.
	Store(guarantee *flow.CollectionGuarantee) error

	// ByID retrieves the collection guarantee by the collection
	// fingerprint.
	ByID(collID flow.Identifier) (*flow.CollectionGuarantee, error)
}
