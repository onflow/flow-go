package types

import "github.com/dapperlabs/flow-go/model/flow"

type BlockProposal struct {
	Block     *Block
	Signature *Signature // this is sign(CompleteBlock)
}

func NewBlockProposal(block *Block, sig *Signature) *BlockProposal {
	return &BlockProposal{
		Block:     block,
		Signature: sig,
	}
}

func (b *BlockProposal) QC() *QuorumCertificate   { return b.Block.QC }
func (b *BlockProposal) View() uint64             { return b.Block.View }
func (b *BlockProposal) BlockID() flow.Identifier { return b.Block.ID() }
func (b *BlockProposal) Height() uint64           { return b.Block.Height }
