package forks

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
)

// BlockContainer wraps a block proposal to implement forest.Vertex
// so the proposal can be stored in forest.LevelledForest
type BlockContainer model.Block

var _ forest.Vertex = (*BlockContainer)(nil)

func ToBlockContainer2(block *model.Block) *BlockContainer { return (*BlockContainer)(block) }
func (b *BlockContainer) Block() *model.Block              { return (*model.Block)(b) }

// Functions implementing forest.Vertex
func (b *BlockContainer) VertexID() flow.Identifier { return b.BlockID }
func (b *BlockContainer) Level() uint64             { return b.View }

func (b *BlockContainer) Parent() (flow.Identifier, uint64) {
	// Caution: not all blocks have a QC for the parent, such as the spork root blocks.
	// Per API contract, we are obliged to return a value to prevent panics during logging.
	// (see vertex `forest.VertexToString` method).
	if b.QC == nil {
		return flow.ZeroID, 0
	}
	return b.QC.BlockID, b.QC.View
}
