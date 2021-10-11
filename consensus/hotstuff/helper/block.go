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

func WithParentSigners(signerIDs []flow.Identifier) func(*model.Block) {
	return func(block *model.Block) {
		block.QC.SignerIDs = signerIDs
	}
}

func MakeProposal(options ...func(*model.Proposal)) *model.Proposal {
	proposal := &model.Proposal{
		Block:   MakeBlock(),
		SigData: unittest.SignatureFixture(),
	}
	for _, option := range options {
		option(proposal)
	}
	return proposal
}

func WithBlock(block *model.Block) func(*model.Proposal) {
	return func(proposal *model.Proposal) {
		proposal.Block = block
	}
}

func WithSigData(sigData []byte) func(*model.Proposal) {
	return func(proposal *model.Proposal) {
		proposal.SigData = sigData
	}
}
