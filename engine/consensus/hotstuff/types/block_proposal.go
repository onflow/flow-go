package types

type BlockProposal struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
	Signature        *Signature
}

func (b *BlockProposal) QC() *QuorumCertificate { return b.Block.QC }
func (b *BlockProposal) View() uint64           { return b.Block.View }
func (b *BlockProposal) BlockMRH() []byte       { return b.Block.BlockMRH() }
