package helper

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func MakeBlock(t *testing.T, options ...func(*model.Block)) *model.Block {
	view := rand.Uint64()
	block := model.Block{
		View:        rand.Uint64(),
		BlockID:     unittest.IdentifierFixture(),
		PayloadHash: unittest.IdentifierFixture(),
		ProposerID:  unittest.IdentifierFixture(),
		Timestamp:   time.Now().UTC(),
		QC:          MakeQC(t, WithQCView(view-1)),
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

func WithBlockProposerID(proposerID flow.Identifier) func(*model.Block) {
	return func(block *model.Block) {
		block.ProposerID = proposerID
	}
}

func WithQCBlock(parent *model.Block) func(*model.Block) {
	return func(block *model.Block) {
		block.QC.BlockID = parent.BlockID
		block.QC.View = parent.View
	}
}

func WithQCSignerIDs(signerIDs []flow.Identifier) func(*model.Block) {
	return func(block *model.Block) {
		block.QC.SignerIDs = signerIDs
	}
}
