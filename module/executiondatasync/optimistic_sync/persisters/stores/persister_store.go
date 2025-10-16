package stores

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/storage"
)

// PersisterStore is the interface to handle persisting of a data type to persisted storage using batch operation.
type PersisterStore interface {
	// Persist adds data to the batch for later commitment.
	//
	// No error returns are expected during normal operations
	Persist(lctx lockctx.Proof, batch storage.ReaderBatchWriter) error
}
