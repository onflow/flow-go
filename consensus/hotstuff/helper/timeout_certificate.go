package helper

import (
	"math/rand"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func MakeTC(options ...func(*flow.TimeoutCertificate)) *flow.TimeoutCertificate {
	qc := MakeQC()
	signerIndices := unittest.SignerIndicesFixture(3)
	highQCViews := make([]uint64, 3)
	for i := range highQCViews {
		highQCViews[i] = qc.View
	}
	tc := flow.TimeoutCertificate{
		View:          rand.Uint64(),
		NewestQC:      qc,
		NewestQCViews: []uint64{qc.View},
		SignerIndices: signerIndices,
		SigData:       unittest.SignatureFixture(),
	}
	for _, option := range options {
		option(&tc)
	}
	return &tc
}

func WithTCNewestQC(qc *flow.QuorumCertificate) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.NewestQC = qc
		tc.NewestQCViews = []uint64{qc.View}
	}
}

func WithTCSigners(signerIndices []byte) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.SignerIndices = signerIndices
	}
}

func WithTCView(view uint64) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.View = view
	}
}

func WithTCHighQCViews(highQCViews []uint64) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.NewestQCViews = highQCViews
	}
}

func TimeoutObjectFixture(opts ...func(TimeoutObject *hotstuff.TimeoutObject)) *hotstuff.TimeoutObject {
	timeout := &hotstuff.TimeoutObject{
		View:       uint64(rand.Uint32()),
		NewestQC:   MakeQC(),
		LastViewTC: MakeTC(),
		SignerID:   unittest.IdentifierFixture(),
		SigData:    unittest.RandomBytes(128),
	}

	for _, opt := range opts {
		opt(timeout)
	}

	return timeout
}

func WithTimeoutObjectSignerID(signerID flow.Identifier) func(*hotstuff.TimeoutObject) {
	return func(TimeoutObject *hotstuff.TimeoutObject) {
		TimeoutObject.SignerID = signerID
	}
}

func WithTimeoutNewestQC(newestQC *flow.QuorumCertificate) func(*hotstuff.TimeoutObject) {
	return func(timeout *hotstuff.TimeoutObject) {
		timeout.NewestQC = newestQC
	}
}

func WithTimeoutLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*hotstuff.TimeoutObject) {
	return func(timeout *hotstuff.TimeoutObject) {
		timeout.LastViewTC = lastViewTC
	}
}

func WithTimeoutObjectView(view uint64) func(*hotstuff.TimeoutObject) {
	return func(TimeoutObject *hotstuff.TimeoutObject) {
		TimeoutObject.View = view
	}
}
