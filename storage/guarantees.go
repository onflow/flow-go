package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Guarantees represents persistent storage for collection guarantees.
type Guarantees interface {

	// Save inserts the collection guarantee.
	Save(guarantee *flow.CollectionGuarantee)

	// ByFingerprint retrieves the collection guarantee by the collection
	// fingerprint.
	ByFingerPrint(hash flow.Fingerprint) (*flow.CollectionGuarantee, error)
}
