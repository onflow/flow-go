package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkStatuses is an implementation of in-memory storage for maintaining the chunk status data objects.
// Stored ChunkStatuses are keyed by chunks.Locator ID.
type ChunkStatuses struct {
	*Backend[flow.Identifier, *verification.ChunkStatus]
}

func NewChunkStatuses(limit uint) *ChunkStatuses {
	return &ChunkStatuses{NewBackend(WithLimit[flow.Identifier, *verification.ChunkStatus](limit))}
}
