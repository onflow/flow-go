// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChunkStates represents a concurrency-safe memory pool for execution states.
// Each item in the pool represents a subset of execution state for a
// particular chunk.
type ChunkStates interface {

	// Has checks whether the chunk state for the given chunk is currently in
	// the memory pool.
	Has(chunkID flow.Identifier) bool

	// Add will add the given chunk state to the memory pool; it will error if
	// the chunk state is already in the memory pool.
	Add(chunkState *flow.ChunkState) error

	// Rem will remove the given chunk state from the memory pool; it will
	// return true if the chunk state was known and removed.
	Rem(chunkID flow.Identifier) bool

	// Get will retrieve the given chunk state from the memory pool; it will
	// error if the chunk state is not in the memory pool.
	ByID(chunkID flow.Identifier) (*flow.ChunkState, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all chunk states that are currently in the memory pool
	// as a slice.
	All() []*flow.ChunkState

	// Hash will return a hash of the contents of the memory pool.
	Hash() flow.Identifier
}
