package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Guarantees represents persistent storage for collection guarantees.
type Guarantees interface {

	// Save inserts the collection guarantee.
	Save(guarantee *flow.CollectionGuarantee) error

	// ByFingerprint retrieves the collection guarantee by the collection
	// fingerprint.
	ByFingerprint(hash flow.Fingerprint) (*flow.CollectionGuarantee, error)
}
