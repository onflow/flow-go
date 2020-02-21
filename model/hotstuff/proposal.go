package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Proposal represent a new proposed block within HotStuff (and thus a
// a header in the bigger picture), signed by the proposer.
type Proposal struct {
	Block     *Block
	Signature crypto.Signature
}

// ProposerVote extracts the proposer vote from the proposal
func (p *Proposal) ProposerVote() *Vote {
	sig := SingleSignature{
		SignerID: p.Block.ProposerID,
		Raw:      p.Signature,
	}
	vote := Vote{
		BlockID:   p.Block.BlockID,
		View:      p.Block.View,
		Signature: &sig,
	}
	return &vote
}

// ProposalFromFlow turns a flow header into a hotstuff block type.
func ProposalFromFlow(header *flow.Header, parentView uint64) *Proposal {
	sig := AggregatedSignature{
		Raw:       header.ParentSigs,
		SignerIDs: header.ParentSigners,
	}
	qc := QuorumCertificate{
		BlockID:             header.ParentID,
		View:                parentView,
		AggregatedSignature: &sig,
	}
	block := Block{
		BlockID:     header.ID(),
		View:        header.View,
		QC:          &qc,
		ProposerID:  header.ProposerID,
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}
	proposal := Proposal{
		Block:     &block,
		Signature: header.ProposerSig,
	}
	return &proposal
}

// ProposalToFlow turns a block proposal into a flow header.
func ProposalToFlow(proposal *Proposal) *flow.Header {
	block := proposal.Block
	header := flow.Header{
		ParentID:      block.QC.BlockID,
		PayloadHash:   block.PayloadHash,
		Timestamp:     block.Timestamp,
		View:          block.View,
		ParentSigners: block.QC.AggregatedSignature.SignerIDs,
		ParentSigs:    block.QC.AggregatedSignature.Raw,
	}
	return &header
}
