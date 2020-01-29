package types

import "github.com/dapperlabs/flow-go/model/flow"

// BlockHeader is a temporary type for the abstraction of block proposal that hotstuff
// received from the outside network. Will be placed
type BlockHeader struct {
	Block            *Block
	ConsensusPayload *ConsensusPayload
	Signature        *Signature // CAUTION: this is sign(Block), i.e. it does NOT include ConsensusPayload
}

func NewBlockHeader(block *Block, consensusPayload *ConsensusPayload, sig *Signature) *BlockProposal {
	return &BlockProposal{
		Block:            block,
		ConsensusPayload: consensusPayload,
		Signature:        sig,
	}
}

func (b BlockHeader) QC() *QuorumCertificate    { return b.Block.QC }
func (b BlockHeader) View() uint64              { return b.Block.View }
func (b BlockHeader) BlockMRH() flow.Identifier { return b.Block.BlockMRH() }
func (b BlockHeader) Height() uint64            { return b.Block.Height }

// ToVote converts a BlockProposal to a Vote
func (b BlockHeader) ToVote() *Vote {
	return NewVote(NewUnsignedVote(b.View(), b.BlockMRH()), b.Signature)
}
