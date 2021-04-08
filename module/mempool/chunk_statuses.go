package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkStatuses is an in-memory storage for maintaining the chunk status data objects.
type ChunkStatuses interface {
	ByID(chunkID flow.Identifier) (*verification.ChunkStatus, bool)
	Add(status *verification.ChunkStatus) bool
	Rem(chunkID flow.Identifier) bool
	All() []*verification.ChunkStatus
}
