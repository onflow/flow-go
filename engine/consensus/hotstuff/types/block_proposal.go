package types

type BlockProposal struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
	Signature        *Signature
}

func (p *BlockProposal) View() uint64 {
	return p.Block.View
}

func (p *BlockProposal) ID() MRH {
	return p.Block.BlockMRH()
}

func (p *BlockProposal) Level() uint64 {
	return p.View()
}

func (p *BlockProposal) Parent() (MRH, uint64) {
	return p.Block.QC.BlockMRH, p.Block.QC.View
}
