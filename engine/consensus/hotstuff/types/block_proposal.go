package types

type BlockProposal struct {
	Block            *Block
	ConsensusPayload ConsensusPayload
	Signature        Signature
}
