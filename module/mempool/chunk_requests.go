package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

type ChunkRequests interface {
	ByID(chunkID flow.Identifier) (*verification.ChunkRequestStatus, bool)
	Add(chunk *verification.ChunkRequestStatus) bool
	Rem(chunkID flow.Identifier) bool
	IncrementAttempt(chunkID flow.Identifier) bool
	All() []*verification.ChunkRequestStatus
}
