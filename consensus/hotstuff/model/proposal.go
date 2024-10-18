package model

import (
	"github.com/onflow/flow-go/model/flow"
)

// Proposal represents a block proposal under construction.
// In order to decide whether a proposal is safe to sign, HotStuff's Safety Rules require
// proof that the leader entered the respective view in a protocol-compliant manner. Specifically,
// we require the QC (mandatory part of every block) and optionally a TC (of the QC in the block
// is _not_ for the immediately previous view). Note that the proposer's signature for its own
// block is _not_ required.
// By explicitly differentiating the Proposal from the SignedProposal (extending Proposal by
// adding the proposer's signature), we can unify the algorithmic path of signing block proposals.
// This codifies the important aspect that a proposer's signature for their own block
// is conceptually also just a vote (we explicitly use that for aggregating votes, including the
// proposer's own vote to a QC). In order to express this conceptual equivalence in code, the
// voting logic in Safety Rules must also operate on an unsigned Proposal.
type Proposal struct {
	Block      *Block
	LastViewTC *flow.TimeoutCertificate
}

// SignedProposal represent a new proposed block within HotStuff (and thus
// a header in the bigger picture), signed by the proposer.
type SignedProposal struct {
	Proposal
	SigData []byte
}

// ProposerVote extracts the proposer vote from the proposal
func (p *SignedProposal) ProposerVote() *Vote {
	vote := Vote{
		View:     p.Block.View,
		BlockID:  p.Block.BlockID,
		SignerID: p.Block.ProposerID,
		SigData:  p.SigData,
	}
	return &vote
}

// SignedProposalFromFlow turns a flow header into a hotstuff block type.
func SignedProposalFromFlow(header *flow.Header) *SignedProposal {
	proposal := SignedProposal{
		Proposal: Proposal{
			Block:      BlockFromFlow(header),
			LastViewTC: header.LastViewTC,
		},
		SigData: header.ProposerSigData,
	}
	return &proposal
}

// ProposalFromFlow turns an unsigned flow header into a unsigned hotstuff block type.
func ProposalFromFlow(header *flow.Header) *Proposal {
	proposal := Proposal{
		Block:      BlockFromFlow(header),
		LastViewTC: header.LastViewTC,
	}
	return &proposal
}

// SignedProposalToFlow turns a block proposal into a flow header.
func SignedProposalToFlow(proposal *SignedProposal) *flow.Header {

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
