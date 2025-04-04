package unsynchronized

import (
	"github.com/onflow/flow-go/storage"
)

type Persister interface {
	AddToBatch(batch storage.ReaderBatchWriter) error
}
