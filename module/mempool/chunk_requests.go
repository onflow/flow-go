package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkRequests is an in-memory storage for maintaining chunk requests data objects.
type ChunkRequests interface {
	ByID(chunkID flow.Identifier) (*verification.ChunkRequestStatus, bool)
	Add(request *verification.ChunkRequestStatus) bool
	Rem(chunkID flow.Identifier) bool
	IncrementAttempt(chunkID flow.Identifier) bool
	All() []*verification.ChunkRequestStatus
}
