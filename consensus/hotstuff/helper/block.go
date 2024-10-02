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

func MakeProposal(options ...func(*model.SignedProposal)) *model.SignedProposal {
	proposal := &model.SignedProposal{
		Proposal: model.Proposal{
			Block:      MakeBlock(),
			LastViewTC: nil,
		},
		SigData: unittest.SignatureFixture(),
	}
	for _, option := range options {
		option(proposal)
	}
	return proposal
}

func WithBlock(block *model.Block) func(*model.SignedProposal) {
	return func(proposal *model.SignedProposal) {
		proposal.Block = block
	}
}

func WithSigData(sigData []byte) func(*model.SignedProposal) {
	return func(proposal *model.SignedProposal) {
		proposal.SigData = sigData
	}
}

func WithLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*model.SignedProposal) {
	return func(proposal *model.SignedProposal) {
		proposal.LastViewTC = lastViewTC
	}
}
