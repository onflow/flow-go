package helper

import (
	"math/rand"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func MakeBlock(options ...func(*model.Block)) *model.Block {
	view := rand.Uint64()
	block := model.Block{
		View:        view,
		BlockID:     unittest.IdentifierFixture(),
		PayloadHash: unittest.IdentifierFixture(),
		ProposerID:  unittest.IdentifierFixture(),
		Timestamp:   time.Now().UTC(),
		QC:          MakeQC(WithQCView(view - 1)),
	}
	for _, option := range options {
		option(&block)
	}
	return &block
}

func WithBlockView(view uint64) func(*model.Block) {
	return func(block *model.Block) {
		block.View = view
	}
}

func WithBlockProposer(proposerID flow.Identifier) func(*model.Block) {
	return func(block *model.Block) {
		block.ProposerID = proposerID
	}
}

func WithParentBlock(parent *model.Block) func(*model.Block) {
	return func(block *model.Block) {
		block.QC.BlockID = parent.BlockID
		block.QC.View = parent.View
	}
}

func WithParentSigners(signerIndices []byte) func(*model.Block) {
	return func(block *model.Block) {
		block.QC.SignerIndices = signerIndices
	}
}

func WithBlockQC(qc *flow.QuorumCertificate) func(*model.Block) {
	return func(block *model.Block) {
		block.QC = qc
	}
}

func MakeSignedProposal(options ...func(*model.SignedProposal)) *model.SignedProposal {
	proposal := &model.SignedProposal{
		Proposal: *MakeProposal(),
		SigData:  unittest.SignatureFixture(),
	}
	for _, option := range options {
		option(proposal)
	}
	return proposal
}

func MakeProposal(options ...func(*model.Proposal)) *model.Proposal {
	proposal := &model.Proposal{
		Block:      MakeBlock(),
		LastViewTC: nil,
	}
	for _, option := range options {
		option(proposal)
	}
	return proposal
}

func WithProposal(proposal *model.Proposal) func(*model.SignedProposal) {
	return func(signedProposal *model.SignedProposal) {
		signedProposal.Proposal = *proposal
	}
}

func WithBlock(block *model.Block) func(*model.Proposal) {
	return func(proposal *model.Proposal) {
		proposal.Block = block
	}
}

func WithSigData(sigData []byte) func(*model.SignedProposal) {
	return func(proposal *model.SignedProposal) {
		proposal.SigData = sigData
	}
}

func WithLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*model.Proposal) {
	return func(proposal *model.Proposal) {
		proposal.LastViewTC = lastViewTC
	}
}

// SignedProposalToFlow turns a HotStuff block proposal into a flow block proposal.
//
// CAUTION: This function is only suitable for TESTING purposes ONLY.
// In the conversion from `flow.Header` to HotStuff's `model.Block` we lose information
// (e.g. `ChainID` and `Height` are not included in `model.Block`) and hence the conversion
// is *not reversible*. This is on purpose, because we wanted to only expose data to
// HotStuff that HotStuff really needs.
func SignedProposalToFlow(proposal *model.SignedProposal) *flow.ProposalHeader {
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
		LastViewTC:         proposal.LastViewTC,
	}

	return &flow.ProposalHeader{
		Header:          header,
		ProposerSigData: proposal.SigData,
	}
}
