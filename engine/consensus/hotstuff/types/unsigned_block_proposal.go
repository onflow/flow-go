package types

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

func (u *UnsignedBlockProposal) View() uint64     { return u.Block.View }
func (u *UnsignedBlockProposal) BlockMRH() []byte { return u.Block.BlockMRH() }

// BytesForSig returns the bytes for signing.
// Since the signature for a proposal also serves as a vote, in order to do that
// the bytes for signing a block proposal should be the same as the bytes for signing
// a vote
func (u *UnsignedBlockProposal) BytesForSig() []byte {
	// making a unsigned vote to ensure the signature on a proposal can be used as a vote
	vote := NewUnsignedVote(u.View(), u.BlockMRH())
	return vote.BytesForSig()
}
