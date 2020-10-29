package helper

import (
	"math/rand"
	"testing"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func MakeQC(t *testing.T, options ...func(*flow.QuorumCertificate)) *flow.QuorumCertificate {
	qc := flow.QuorumCertificate{
		View:      rand.Uint64(),
		BlockID:   unittest.IdentifierFixture(),
		SignerIDs: unittest.IdentityListFixture(7).NodeIDs(),
		SigData:   unittest.SignatureFixture(),
	}
	for _, option := range options {
		option(&qc)
	}
	return &qc
}

func WithQCBlock(block *model.Block) func(*flow.QuorumCertificate) {
	return func(qc *flow.QuorumCertificate) {
		qc.View = block.View
		qc.BlockID = block.BlockID
	}
}

func WithQCSigners(signerIDs []flow.Identifier) func(*flow.QuorumCertificate) {
	return func(qc *flow.QuorumCertificate) {
		qc.SignerIDs = signerIDs
	}
}

func WithQCView(view uint64) func(*flow.QuorumCertificate) {
	return func(qc *flow.QuorumCertificate) {
		qc.View = view
	}
}
