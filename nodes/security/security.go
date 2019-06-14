package security

import (
	"context"

	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/nodes/security/block_builder"
)

// Node simulates the behaviour of a Bamboo security node.
type Node struct {
	state         *data.WorldState
	collectionsIn chan *data.Collection
	blockBuilder  *block_builder.BlockBuilder
}

// NewNode returns a new simulated security node.
func NewNode(state *data.WorldState, collectionsIn chan *data.Collection) *Node {
	blockBuilder := block_builder.NewBlockBuilder(state, collectionsIn)

	return &Node{
		state:         state,
		collectionsIn: collectionsIn,
		blockBuilder:  blockBuilder,
	}
}

func (n *Node) Start(ctx context.Context) {
	n.blockBuilder.Start(ctx)
}
