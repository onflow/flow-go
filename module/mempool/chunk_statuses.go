package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkStatuses is an in-memory storage for maintaining the chunk status data objects.
type ChunkStatuses interface {
	// Get returns a chunk status by its chunk index and result ID.
	// There is a one-to-one correspondence between the chunk statuses in memory, and
	// their pair of chunk index and result id.
	Get(chunkIndex uint64, chunkID flow.Identifier) (*verification.ChunkStatus, bool)

	// Add provides insertion functionality into the memory pool.
	// The insertion is only successful if there is no duplicate status with the same
	// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
	Add(status *verification.ChunkStatus) bool

	// Rem provides deletion functionality from the memory pool.
	// If there is a chunk status with this ID, Rem removes it and returns true.
	// Otherwise it returns false.
	Rem(chunkID flow.Identifier) bool

	// All returns all chunk statuses stored in this memory pool.
	All() []*verification.ChunkStatus

	// Size returns total number of chunk statuses in the memory pool.
	Size() uint
}
