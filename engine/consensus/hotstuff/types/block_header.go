package types

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockHeader is a temporary type for the abstraction of block proposal that hotstuff
// received from the outside network. Will be placed with
// TODO: type BlockHeader flow.Header
type BlockHeader BlockProposal

func NewBlockHeader(block *Block, sig *flow.PartialSignature) *BlockHeader {
	return &BlockHeader{
		Block:     block,
		Signature: sig,
	}
}

// BlockHeaderFromFlow converts a flow header to the corresponding internal
// HotStuff block proposal.
func BlockHeaderFromFlow(header *flow.Header) *BlockHeader {
	block := BlockFromFlowHeader(header)
	return NewBlockHeader(block, header.ProposerSig)
}

func (b BlockHeader) QC() *QuorumCertificate   { return b.Block.QC }
func (b BlockHeader) View() uint64             { return b.Block.View }
func (b BlockHeader) BlockID() flow.Identifier { return b.Block.BlockID }
func (b BlockHeader) Height() uint64           { return b.Block.Height }
