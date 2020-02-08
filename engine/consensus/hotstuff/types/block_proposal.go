package types

import "github.com/dapperlabs/flow-go/model/flow"

type BlockProposal struct {
	Block     *Block
	Signature *flow.PartialSignature // this is sign(Block)
}

func NewBlockProposal(block *Block, sig *flow.PartialSignature) *BlockProposal {
	return &BlockProposal{
		Block:     block,
		Signature: sig,
	}
}

func (b *BlockProposal) QC() *QuorumCertificate   { return b.Block.QC }
func (b *BlockProposal) View() uint64             { return b.Block.View }
func (b *BlockProposal) BlockID() flow.Identifier { return b.Block.BlockID }
func (b *BlockProposal) Height() uint64           { return b.Block.Height }

func (b *BlockProposal) ToVote() *Vote {
	panic("TODO")
}
