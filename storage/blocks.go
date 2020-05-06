// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package storage

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Blocks represents persistent storage for blocks.
type Blocks interface {

	// Store stores the block. If the exactly same block is already in a storage, return successfully
	Store(block *flow.Block) error

	// ByID returns the block with the given hash. It is available for
	// finalized and ambiguous blocks.
	ByID(blockID flow.Identifier) (*flow.Block, error)

	// ByHeight returns the block at the given height. It is only available
	// for finalized blocks.
	ByHeight(height uint64) (*flow.Block, error)

	// ByCollectionID returns the block for the given collection ID.
	ByCollectionID(collID flow.Identifier) (*flow.Block, error)
}
