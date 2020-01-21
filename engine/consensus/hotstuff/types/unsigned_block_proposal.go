package types

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

func (u *UnsignedBlockProposal) BytesForSig() []byte {
	return u.Block.BlockMRH()
}
