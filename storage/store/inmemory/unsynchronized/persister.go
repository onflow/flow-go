package unsynchronized

import (
	"github.com/onflow/flow-go/storage"
)

// Persister is responsible for persisting in-memory storages to the given DB.
type Persister interface {
	// AddToBatch adds all the in-memory storages to the given batch.
	// It is used for the batching writes to the DB.
	AddToBatch(batch storage.ReaderBatchWriter) error
}
