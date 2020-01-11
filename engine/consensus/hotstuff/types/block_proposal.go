package types

type BlockProposal struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
	Signature        *Signature
}

func NewBlockProposal(block *Block, consensusPayload *ConsensusPayload) *BlockProposal {
	return &BlockProposal{
		Block:            block,
		ConsensusPayload: consensusPayload,
	}
}
