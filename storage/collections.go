package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Collections represents persistent storage for collections.
type Collections interface {
	// ByHash returns the transaction for the given block.
	ByFingerprint(hash flow.Fingerprint) (*flow.Collection, error)

	// Save inserts the transaction, keyed by hash.
	Save(collection *flow.Collection) error
}
