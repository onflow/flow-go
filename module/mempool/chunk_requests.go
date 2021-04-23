package mempool

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkRequests is an in-memory storage for maintaining chunk data pack requests.
type ChunkRequests interface {
	// ByID returns a chunk request by its chunk ID.
	//
	// There is a one-to-one correspondence between the chunk requests in memory, and
	// their chunk ID.
	ByID(chunkID flow.Identifier) (*verification.ChunkDataPackRequest, bool)

	// RequestInfo returns the number of times the chunk has been requested,
	// last time the chunk has been requested, and the retry-after time duration of the
	// underlying request status of this chunk.
	//
	// The last boolean parameter returns whether a chunk request for this chunk ID
	// exists in memory-pool.
	RequestInfo(chunkID flow.Identifier) (uint64, time.Time, time.Duration, bool)

	// Add provides insertion functionality into the memory pool.
	// The insertion is only successful if there is no duplicate chunk request with the same
	// chunk ID in the memory. Otherwise, it aborts the insertion and returns false.
	Add(request *verification.ChunkDataPackRequest) bool

	// Rem provides deletion functionality from the memory pool.
	// If there is a chunk request with this ID, Rem removes it and returns true.
	// Otherwise it returns false.
	Rem(chunkID flow.Identifier) bool

	// IncrementAttempt increments the Attempt field of the corresponding status of the
	// chunk request in memory pool that has the specified chunk ID.
	// If such chunk ID does not exist in the memory pool, it returns false.
	//
	// The increments are done atomically, thread-safe, and in isolation.
	IncrementAttempt(chunkID flow.Identifier) bool

	// UpdateRetryAfter updates the retryAfter field of the chunk request to the specified values.
	// It also increments the number of time this chunk has been attempted, and the last time this chunk
	// has been attempted to the current time.
	//
	// If such chunk ID does not exist in the memory pool, it returns false.
	// The updates under this method are atomic, thread-safe, and done in isolation.
	UpdateRetryAfter(chunkID flow.Identifier, retryAfter time.Duration) bool

	// All returns all chunk requests stored in this memory pool.
	All() []*verification.ChunkDataPackRequest
}
