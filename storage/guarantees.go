package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

// Guarantees represents persistent storage for collection guarantees.
// Must only be used to store finalized collection guarantees.
type Guarantees interface {

	// Store inserts the collection guarantee and indexes it by the collection ID.
	// It is the caller's responsibility to ensure that only finalized collection guarantees are passed to be stored.
	// Expected errors of the returned anonymous function:
	//   - storage.ErrDataMismatch if a different guarantee for the same collection is already stored.
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
