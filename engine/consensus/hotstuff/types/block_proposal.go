package types

import "github.com/dapperlabs/flow-go/model/flow"

type BlockProposal struct {
	Block     *Block
	Signature *Signature
}

func NewBlockProposal(block *Block, sig *Signature) *BlockProposal {
	return &BlockProposal{
		Block:     block,
		Signature: sig,
	}
}

// QC is the QuorumCertificate of a Block
func (b BlockProposal) QC() *QuorumCertificate { return b.Block.QC }

// View is the view of the Block
func (b BlockProposal) View() uint64 { return b.Block.View }

// BlockID is the Merkle Root Hash of a Block
func (b BlockProposal) BlockID() flow.Identifier { return b.Block.BlockID() }

// Height is the height of the Block. It equals to the parent block's height + 1
func (b BlockProposal) Height() uint64 { return b.Block.Height }

// ToVote converts a BlockProposal to a Vote
func (b BlockProposal) ToVote() *Vote {
	return NewVote(NewUnsignedVote(b.View(), b.BlockID()), b.Signature)
}
