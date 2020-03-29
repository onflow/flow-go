package helper

import (
	"math/rand"
	"testing"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func MakeQC(t *testing.T, options ...func(*model.QuorumCertificate)) *model.QuorumCertificate {
	qc := model.QuorumCertificate{
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

func WithQCView(view uint64) func(*model.QuorumCertificate) {
	return func(qc *model.QuorumCertificate) {
		qc.View = view
	}
}
