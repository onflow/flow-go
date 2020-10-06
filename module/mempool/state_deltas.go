// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// Deltas represents a concurrency-safe memory pool for block deltas.
type Deltas interface {

	// Has checks whether the block delta with the given hash is currently in
	// the memory pool.
	Has(blockID flow.Identifier) bool

	// Add will add the given block delta to the memory pool. It will return
	// false if it was already in the mempool.
	Add(delta *messages.ExecutionStateDelta) bool

	// Rem will remove the given block delta from the memory pool; it will
	// will return true if the block delta was known and removed.
	Rem(blockID flow.Identifier) bool

	// ByID retrieve the block delta with the given ID from the memory
	// pool. It will return false if it was not found in the mempool.
	ByBlockID(blockID flow.Identifier) (*messages.ExecutionStateDelta, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// Limit will return the maximum size of the memory pool
	Limit() uint

	// All will retrieve all block deltas that are currently in the memory pool
	// as a slice.
	All() []*messages.ExecutionStateDelta

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
