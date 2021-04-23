package mempool

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
)

// ChunkRequests is an in-memory storage for maintaining chunk data pack requests.
type ChunkRequests interface {
	// ByID returns a chunk request by its chunk ID as well as the attempt field of its underlying
	// chunk request status.
	// There is a one-to-one correspondence between the chunk requests in memory, and
	// their chunk ID.
	ByID(chunkID flow.Identifier) (*verification.ChunkDataPackRequest, int, bool)

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

	// IncrementAttemptAndRetryAfter increments the Attempt field of the chunk request in memory pool that
	// has the specified chunk ID, and updates the retryAfter field of the chunk request to the specified
	// values.
	//
	// The LastAttempt field of the chunk request is timestamped with the invocation time of this method.
	//
	// If such chunk ID does not exist in the memory pool, it returns false.
	// The updates under this method are atomic, thread-safe, and done in isolation.
	IncrementAttemptAndRetryAfter(chunkID flow.Identifier, retryAfter time.Duration) bool

	// All returns all chunk requests stored in this memory pool.
	All() []*verification.ChunkDataPackRequest
}
