package unsynchronized

import (
	"github.com/onflow/flow-go/storage"
)

// Persister is responsible for persisting in-memory storages to given DB.
type Persister interface {
	AddToBatch(batch storage.ReaderBatchWriter) error
}
