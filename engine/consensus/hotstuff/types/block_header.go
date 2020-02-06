package types

import "github.com/dapperlabs/flow-go/model/flow"

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
func BlockHeaderFromFlow(header *flow.Header) *BlockHeader {
	return &BlockHeader{
		Block: &Block{
			View: header.View,
			QC: &QuorumCertificate{
				View:                header.ParentView,
				BlockID:             header.ParentID,
				AggregatedSignature: header.ParentSig,
			},
			PayloadHash: header.PayloadHash[:],
			Height:      header.Number,
			ChainID:     header.ChainID,
			Timestamp:   header.Timestamp,
		},
		Signature: header.ProposerSig,
	}
}

func (b BlockHeader) QC() *QuorumCertificate   { return b.Block.QC }
func (b BlockHeader) View() uint64             { return b.Block.View }
func (b BlockHeader) BlockID() flow.Identifier { return b.Block.ID() }
func (b BlockHeader) Height() uint64           { return b.Block.Height }
