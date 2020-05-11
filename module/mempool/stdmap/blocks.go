// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Blocks implements the blocks memory pool.
type Blocks struct {
	*Backend
}

// NewBlocks creates a new memory pool for blocks.
func NewBlocks(limit uint) (*Blocks, error) {
	a := &Blocks{
		Backend: NewBackend(WithLimit(limit)),
	}

	return a, nil
}

// Add adds an block to the mempool.
func (a *Blocks) Add(block *flow.Block) bool {
	return a.Backend.Add(block)
}

// ByID returns the block with the given ID from the mempool.
func (a *Blocks) ByID(blockID flow.Identifier) (*flow.Block, bool) {
	entity, exists := a.Backend.ByID(blockID)
	if !exists {
		return nil, false
	}
	block := entity.(*flow.Block)
	return block, true
}

// All returns all blocks from the pool.
func (a *Blocks) All() []*flow.Block {
	entities := a.Backend.All()
	blocks := make([]*flow.Block, 0, len(entities))
	for _, entity := range entities {
		blocks = append(blocks, entity.(*flow.Block))
	}
	return blocks
}
