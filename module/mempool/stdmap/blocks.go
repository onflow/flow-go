// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Blocks implements the blocks memory pool.
type Blocks struct {
	*backend
}

// NewBlocks creates a new memory pool for blocks.
func NewBlocks() (*Blocks, error) {
	a := &Blocks{
		backend: newBackend(),
	}

	return a, nil
}

// Add adds an block to the mempool.
func (a *Blocks) Add(approval *flow.Block) error {
	return a.backend.Add(approval)
}

// Get returns the block with the given ID from the mempool.
func (a *Blocks) Get(approvalID flow.Identifier) (*flow.Block, error) {
	entity, err := a.backend.Get(approvalID)
	if err != nil {
		return nil, err
	}
	approval, ok := entity.(*flow.Block)
	if !ok {
		panic(fmt.Sprintf("invalid entity in approval pool (%T)", entity))
	}
	return approval, nil
}

// All returns all blocks from the pool.
func (a *Blocks) All() []*flow.Block {
	entities := a.backend.All()
	Blocks := make([]*flow.Block, 0, len(entities))
	for _, entity := range entities {
		approval, ok := entity.(*flow.Block)
		if !ok {
			panic(fmt.Sprintf("invalid entity in approval pool (%T)", entity))
		}
		Blocks = append(Blocks, approval)
	}
	return Blocks
}
