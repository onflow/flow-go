package types

import "github.com/dapperlabs/flow-go/model/flow"

type BlockProposal struct {
	Block     *Block
	Signature *VoteSignature // CAUTION: this is sign(Block), i.e. it does NOT include ConsensusPayload
}

func NewBlockProposal(block *Block, sig *VoteSignature) *BlockProposal {
	return &BlockProposal{
		Block:     block,
		Signature: sig,
	}
}

func (b *BlockProposal) QC() *QuorumCertificate   { return b.Block.QC }
func (b *BlockProposal) View() uint64             { return b.Block.View }
func (b *BlockProposal) BlockID() flow.Identifier { return b.Block.ID() }
func (b *BlockProposal) Height() uint64           { return b.Block.Height }
