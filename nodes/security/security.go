package security

import (
	"context"

	"github.com/dapperlabs/bamboo-emulator/data"
)

// Node simulates the behaviour of a Bamboo security node.
type Node struct {
	state           *data.WorldState
	collectionsIn   chan *data.Collection
	pendingBlocksIn chan *data.Block
	blockBuilder    *BlockBuilder
}

// NewNode returns a new simulated security node.
func NewNode(state *data.WorldState, collectionsIn chan *data.Collection, pendingBlocksIn chan *data.Block) *Node {
	blockBuilder := NewBlockBuilder(state, collectionsIn, pendingBlocksIn)

	return &Node{
		state:           state,
		collectionsIn:   collectionsIn,
		pendingBlocksIn: pendingBlocksIn,
		blockBuilder:    blockBuilder,
	}
}

func (n *Node) Start(ctx context.Context) {
	n.blockBuilder.Start(ctx)
}
