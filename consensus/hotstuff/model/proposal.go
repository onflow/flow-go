package model

import (
	"github.com/onflow/flow-go/model/flow"
)

// Proposal represent a new proposed block within HotStuff (and thus a
// a header in the bigger picture), signed by the proposer.
type Proposal struct {
	Block      *Block
	SigData    []byte
	LastViewTC *flow.TimeoutCertificate
}

var _ flow.Entity = (*Proposal)(nil)

// ProposerVote extracts the proposer vote from the proposal
func (p *Proposal) ProposerVote() *Vote {
	vote := Vote{
		View:     p.Block.View,
		BlockID:  p.Block.BlockID,
		SignerID: p.Block.ProposerID,
		SigData:  p.SigData,
	}
	return &vote
}

// ID implements flow.Entity interface by returning static Block.BlockID
func (p *Proposal) ID() flow.Identifier {
	return p.Block.BlockID
}

// Checksum implements flow.Entity interface by returning static Block.BlockID
func (p *Proposal) Checksum() flow.Identifier {
	return p.Block.BlockID
}

// ProposalFromFlow turns a flow header into a hotstuff block type.
func ProposalFromFlow(header *flow.Header) *Proposal {

	block := BlockFromFlow(header)

	proposal := Proposal{
		Block:      block,
		SigData:    header.ProposerSigData,
		LastViewTC: header.LastViewTC,
	}

	return &proposal
}

// ProposalToFlow turns a block proposal into a flow header.
func ProposalToFlow(proposal *Proposal) *flow.Header {

	block := proposal.Block
	header := &flow.Header{
		ParentID:           block.QC.BlockID,
		PayloadHash:        block.PayloadHash,
		Timestamp:          block.Timestamp,
		View:               block.View,
		ParentView:         block.QC.View,
		ParentVoterIndices: block.QC.SignerIndices,
		ParentVoterSigData: block.QC.SigData,
		ProposerID:         block.ProposerID,
		ProposerSigData:    proposal.SigData,
		LastViewTC:         proposal.LastViewTC,
	}

	return header
}
