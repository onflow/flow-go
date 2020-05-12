// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Blocks represents a concurrency-safe memory pool for blocks.
type Blocks interface {

	// Has checks whether the block with the given hash is currently in
	// the memory pool.
	Has(blockID flow.Identifier) bool

	// Add will add the given block to the memory pool. It will return
	// false if it was already in the mempool.
	Add(block *flow.Block) bool

	// Rem will remove the given block from the memory pool; it will
	// will return true if the block was known and removed.
	Rem(blockID flow.Identifier) bool

	// ByID retrieve the block with the given ID from the memory pool.
	// It will return false if it was not found in the mempool.
	ByID(blockID flow.Identifier) (*flow.Block, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all blocks that are currently in the memory pool
	// as a slice.
	All() []*flow.Block

	// Hash will return a hash of the contents of the memory pool.
	Hash() flow.Identifier
}
