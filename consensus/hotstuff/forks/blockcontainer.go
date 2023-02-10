package forks

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
)

// BlockContainer wraps a block proposal to implement forest.Vertex
// so the proposal can be stored in forest.LevelledForest
type BlockContainer struct {
	Proposal *model.Proposal
}

var _ forest.Vertex = (*BlockContainer)(nil)

// Functions implementing forest.Vertex

func (b *BlockContainer) VertexID() flow.Identifier { return b.Proposal.Block.BlockID }
func (b *BlockContainer) Level() uint64             { return b.Proposal.Block.View }
func (b *BlockContainer) Parent() (flow.Identifier, uint64) {
	return b.Proposal.Block.QC.BlockID, b.Proposal.Block.QC.View
}
