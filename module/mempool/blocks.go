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

	// Add will add the given block to the memory pool; it will error if
	// the block is already in the memory pool.
	Add(block *flow.Block) error

	// Rem will remove the given block from the memory pool; it will
	// will return true if the block was known and removed.
	Rem(blockID flow.Identifier) bool

	// Get will retrieve the given block from the memory pool; it will
	// error if the block is not in the memory pool.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all blocks that are currently in the memory pool
	// as a slice.
	All() []*flow.Block

	// Hash will return a hash of the contents of the memory pool.
	Hash() flow.Identifier
}
