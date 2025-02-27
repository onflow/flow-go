package stdmap

import (
	"github.com/onflow/flow-go/model/flow"
)

// Blocks implements the blocks memory pool.
type Blocks struct {
	*Backend[flow.Identifier, *flow.Block]
}

// NewBlocks creates a new memory pool for blocks.
func NewBlocks(limit uint) (*Blocks, error) {
	a := &Blocks{
		Backend: NewBackend[flow.Identifier, *flow.Block](WithLimit[flow.Identifier, *flow.Block](limit)),
	}

	return a, nil
}

// Add adds a block to the mempool.
func (a *Blocks) Add(block *flow.Block) bool {
	return a.Backend.Add(block.ID(), block)
}

// ByID returns the block with the given ID from the mempool.
func (a *Blocks) ByID(blockID flow.Identifier) (*flow.Block, bool) {
	block, exists := a.Backend.ByID(blockID)
	if !exists {
		return nil, false
	}
	return block, true
}

// All returns all blocks from the pool.
func (a *Blocks) All() []*flow.Block {
	entities := a.Backend.All()
	blocks := make([]*flow.Block, 0, len(entities))
	for _, block := range entities {
		blocks = append(blocks, block)
	}

	return blocks
}
