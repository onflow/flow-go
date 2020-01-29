package types

import "github.com/dapperlabs/flow-go/model/flow"

// UnsignedBlockProposal is a block proposal without signature
type UnsignedBlockProposal struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
}

func NewUnsignedBlockProposal(block *Block, consensusPayload *ConsensusPayload) *UnsignedBlockProposal {
	return &UnsignedBlockProposal{
		Block:            block,
		ConsensusPayload: consensusPayload,
	}
}

func (u UnsignedBlockProposal) View() uint64              { return u.Block.View }
func (u UnsignedBlockProposal) BlockMRH() flow.Identifier { return u.Block.BlockMRH() }

// ToVote converts a UnsignedBlockProposal to a UnsignedVote
func (u UnsignedBlockProposal) ToVote() *UnsignedVote {
	return NewUnsignedVote(u.View(), u.BlockMRH())
}
