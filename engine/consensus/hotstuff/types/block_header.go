package types

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// BlockHeader is a temporary type for the abstraction of block proposal that hotstuff
// received from the outside network. Will be placed
type BlockHeader BlockProposal

func NewBlockHeader(block *Block, sig *flow.PartialSignature) *BlockHeader {
	return &BlockHeader{
		Block:     block,
		Signature: sig,
	}
}

// BlockHeaderFromFlow converts a flow header to the corresponding internal
// HotStuff block proposal.
// header - the block header
// sig - the signature and identity of the signer, looked up by chain compliance layer
// aggSig - the aggregated signature on the QC and the identities of all the signers, looked up by
// chain compliance layer
func BlockHeaderFromFlow(header *flow.Header, sig *flow.PartialSignature, aggSig *flow.AggregatedSignature) *BlockHeader {
	block := BlockFromFlowHeader(header, aggSig)
	return NewBlockHeader(block, sig)
}

func (b BlockHeader) QC() *QuorumCertificate   { return b.Block.QC }
func (b BlockHeader) View() uint64             { return b.Block.View }
func (b BlockHeader) BlockID() flow.Identifier { return b.Block.BlockID }
func (b BlockHeader) Height() uint64           { return b.Block.Height }
