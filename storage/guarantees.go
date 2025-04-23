package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Guarantees represents persistent storage for collection guarantees.
type Guarantees interface {

	// Store inserts the collection guarantee and indexes it by the collection ID.
	// Expected errors of the returned anonymous function:
	//   - storage.ErrAlreadyExists if a collection guarantee with the given id is already stored.
	Store(guarantee *flow.CollectionGuarantee) error

	// ByID returns the flow.CollectionGuarantee by its ID.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no collection guarantee with the given Identifier is known.
	ByID(guaranteeID flow.Identifier) (*flow.CollectionGuarantee, error)

	// ByCollectionID retrieves the collection guarantee by collection ID.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no collection guarantee has been indexed for the given collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.CollectionGuarantee, error)
}
