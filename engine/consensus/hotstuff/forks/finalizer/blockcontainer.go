package finalizer

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockContainer wraps a block to implement forest.Vertex
// In addition, it holds some additional properties for efficient processing of blocks
// by the Finalizer
type BlockContainer struct {
	block *types.BlockProposal
}

// functions implementing forest.vertex
func (b *BlockContainer) VertexID() flow.Identifier         { return b.ID() }
func (b *BlockContainer) Level() uint64                     { return b.View() }
func (b *BlockContainer) Parent() (flow.Identifier, uint64) { return b.QC().BlockID, b.QC().View }

// ID returns the block's identifier
func (b *BlockContainer) ID() flow.Identifier {
	return b.block.BlockID()
}

// View returns the block's view number
func (b *BlockContainer) View() uint64 { return b.block.View() }

// QC returns the block's embedded QC pointing to the block's parent
// (this is the QC that points to the other end of a 1-chain)
func (b *BlockContainer) QC() *types.QuorumCertificate { return b.block.QC() }

// Block returns the block in the container (or nil of container is empty)
func (b *BlockContainer) Block() *types.BlockProposal { return b.block }
