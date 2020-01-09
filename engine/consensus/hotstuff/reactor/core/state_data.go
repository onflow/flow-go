package core

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

//type BlockReference struct {
//	Hash []byte
//	View uint64
//}

////BlockAsVertex wraps a block to implement forrest.Vertex
//type BlockAsVertex struct {
//	block *types.BlockProposal
//}
//
//func (b *BlockAsVertex) VertexID() []byte         { return b.block.BlockMRH }
//func (b *BlockAsVertex) Level() uint64            { return b.block.View }
//func (b *BlockAsVertex) Parent() ([]byte, uint64) { return b.block.QC.Hash, b.block.QC.View }

// BlockContainer wraps a block to implement forrest.Vertex
// In addition, it holds some additional properties for efficient processing of blocks by the HotStuff logic
type BlockContainer struct {
	block *types.BlockProposal

	// twoChainHead is the QC that forms the head of the 2-chain pointing backwards from this block to the genesis block.
	// Informally TwoChainHead POINTS TO Block's grandparent.
	// In 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6, this is referred to as b''.justify
	twoChainHead *types.QuorumCertificate

	// threeChainHead is the QC that forms the head of the 3-chain pointing backwards from this block to the genesis block.
	// Informally TwoChainHead POINTS TO Block's great-grandparent.
	// In 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6, this is referred to as b'.justify
	threeChainHead *types.QuorumCertificate
}

func (b *BlockContainer) VertexID() []byte         { return b.block.BlockMRH() }
func (b *BlockContainer) Level() uint64            { return b.block.View() }
func (b *BlockContainer) Parent() ([]byte, uint64) { return b.block.QC().BlockMRH, b.block.QC().View }

// Hash returns the block's hash
func (b *BlockContainer) Hash() []byte { return b.block.BlockMRH() }

// View returns the block's view number
func (b *BlockContainer) View() uint64 { return b.block.View() }

// QC returns the block's embedded QC pointing to the block's parent
// (this is the QC that points to the other end of a 1-chain)
func (b *BlockContainer) QC() *types.QuorumCertificate { return b.block.QC() }

// QC returns the block's embedded QC pointing to the block's parent
// (this is the QC that points to the other end of a 1-chain)
func (b *BlockContainer) Block() *types.BlockProposal { return b.block }

// OneChainHead returns the QC that forms the head of the 1-chain pointing backwards from this block to the genesis block.
// Informally OneChainHead POINTS TO Block's parent.
// In 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6, this is referred to as b*.justify
func (b *BlockContainer) OneChainHead() *types.QuorumCertificate { return b.block.QC() }

// TwoChainHead returns the QC that forms the head of the 2-chain pointing backwards from this block to the genesis block.
// Informally TwoChainHead POINTS TO Block's grandparent.
// In 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6, this is referred to as b''.justify
func (b *BlockContainer) TwoChainHead() *types.QuorumCertificate { return b.twoChainHead }

// ThreeChainHead returns the QC that forms the head of the 3-chain pointing backwards from this block to the genesis block.
// Informally TwoChainHead POINTS TO Block's great-grandparent.
// In 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6, this is referred to as b'.justify
func (b *BlockContainer) ThreeChainHead() *types.QuorumCertificate { return b.threeChainHead }
