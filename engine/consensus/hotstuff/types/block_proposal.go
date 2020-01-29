package types

import "github.com/dapperlabs/flow-go/model/flow"

type BlockProposal struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
	Signature        *Signature // sign(View, BlockMRH)
}

func NewBlockProposal(block *Block, consensusPayload *ConsensusPayload, sig *Signature) *BlockProposal {
	return &BlockProposal{
		Block:            block,
		ConsensusPayload: consensusPayload,
		Signature:        sig,
	}
}

func (b BlockProposal) QC() *QuorumCertificate    { return b.Block.QC }
func (b BlockProposal) View() uint64              { return b.Block.View }
func (b BlockProposal) BlockMRH() flow.Identifier { return b.Block.BlockMRH() }
func (b BlockProposal) Height() uint64            { return b.Block.Height }

// ToVote converts a BlockProposal to a Vote
func (b BlockProposal) ToVote() *Vote {
	return NewVote(NewUnsignedVote(b.View(), b.BlockMRH()), b.Signature)
}
