package types

import "github.com/dapperlabs/flow-go/model/flow"

// BlockHeader is a temporary type for the abstraction of block proposal that hotstuff
// received from the outside network. Will be placed
type BlockHeader struct {
	Block     *Block
	Signature *VoteSignature // CAUTION: this is sign(Block), i.e. it does NOT include ConsensusPayload
}

func NewBlockHeader(block *Block, sig *Signature) *BlockProposal {
	return &BlockProposal{
		Block:     block,
		Signature: sig,
	}
}

func (b BlockHeader) QC() *QuorumCertificate   { return b.Block.QC }
func (b BlockHeader) View() uint64             { return b.Block.View }
func (b BlockHeader) BlockID() flow.Identifier { return b.Block.ID() }
func (b BlockHeader) Height() uint64           { return b.Block.Height }
