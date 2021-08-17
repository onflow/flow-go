package model

import (
	"github.com/onflow/flow-go/model/flow"
)

// Proposal represent a new proposed block within HotStuff (and thus a
// a header in the bigger picture), signed by the proposer.
type Proposal struct {
	Block   *Block
	SigData []byte
}

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

// ProposalFromFlow turns a flow header into a hotstuff block type.
func ProposalFromFlow(header *flow.Header, parentView uint64) *Proposal {

	block := BlockFromFlow(header, parentView)

	proposal := Proposal{
		Block:   block,
		SigData: header.ProposerSigData,
	}

	return &proposal
}

// ProposalToFlow turns a block proposal into a flow header.
func ProposalToFlow(proposal *Proposal) *flow.Header {

	block := proposal.Block
	header := flow.Header{
		ParentID:           block.QC.BlockID,
		PayloadHash:        block.PayloadHash,
		Timestamp:          block.Timestamp,
		View:               block.View,
		ParentVoterIDs:     block.QC.SignerIDs,
		ParentVoterSigData: block.QC.SigData,
		ProposerID:         block.ProposerID,
		ProposerSigData:    proposal.SigData,
	}

	return &header
}
