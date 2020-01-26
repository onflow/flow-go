package finalizer

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// BlockContainer wraps a block to implement forrest.Vertex
// In addition, it holds some additional properties for efficient processing of blocks
// by the Finalizer
type BlockContainer struct {
	block *types.BlockProposal
}

func (b *BlockContainer) VertexID() []byte         { return b.block.BlockMRH() }
func (b *BlockContainer) Level() uint64            { return b.View() }
func (b *BlockContainer) Parent() ([]byte, uint64) { return b.block.QC().BlockMRH, b.block.QC().View }

// Hash returns the block's hash
func (b *BlockContainer) ID() []byte { return b.block.BlockMRH() }

// View returns the block's view number
func (b *BlockContainer) View() uint64 { return b.block.View() }

// QC returns the block's embedded QC pointing to the block's parent
// (this is the QC that points to the other end of a 1-chain)
func (b *BlockContainer) QC() *types.QuorumCertificate { return b.block.QC() }

// QC returns the block's embedded QC pointing to the block's parent
// (this is the QC that points to the other end of a 1-chain)
func (b *BlockContainer) Block() *types.BlockProposal { return b.block }

