package storage

import (
	"github.com/onflow/flow-go/model/flow"
)

type Prunable interface {
	// Prune removes all prunable data from the storage.
	Prune(blockID flow.Identifier, batch BatchStorage) error
}
