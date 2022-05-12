package helper

import (
	"math/rand"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func MakeTC(options ...func(*flow.TimeoutCertificate)) *flow.TimeoutCertificate {
	qc := MakeQC()
	signers := unittest.IdentityListFixture(7).NodeIDs()
	highQCViews := make([]uint64, len(signers))
	for i := range highQCViews {
		highQCViews[i] = qc.View
	}
	tc := flow.TimeoutCertificate{
		View:          rand.Uint64(),
		TOHighestQC:   qc,
		TOHighQCViews: highQCViews,
		SignerIDs:     signers,
		SigData:       unittest.SignatureFixture(),
	}
	for _, option := range options {
		option(&tc)
	}
	return &tc
}

func WithTCHighestQC(qc *flow.QuorumCertificate) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.TOHighestQC = qc
		tc.TOHighQCViews = make([]uint64, len(tc.SignerIDs))
		for i := range tc.TOHighQCViews {
			tc.TOHighQCViews[i] = qc.View
		}
	}
}

func WithTCSigners(signerIDs []flow.Identifier) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.SignerIDs = signerIDs
	}
}

func WithTCView(view uint64) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.View = view
	}
}

func WithTCHighQCViews(highQCViews []uint64) func(*flow.TimeoutCertificate) {
	return func(tc *flow.TimeoutCertificate) {
		tc.TOHighQCViews = highQCViews
	}
}

func TimeoutObjectFixture(opts ...func(TimeoutObject *hotstuff.TimeoutObject)) *hotstuff.TimeoutObject {
	timeout := &hotstuff.TimeoutObject{
		View:       uint64(rand.Uint32()),
		HighestQC:  MakeQC(),
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

func WithTimeoutObjectView(view uint64) func(*hotstuff.TimeoutObject) {
	return func(TimeoutObject *hotstuff.TimeoutObject) {
		TimeoutObject.View = view
	}
}

func TimeoutObjectWithStakingSig() func(*hotstuff.TimeoutObject) {
	return func(TimeoutObject *hotstuff.TimeoutObject) {
		TimeoutObject.SigData = unittest.RandomBytes(128)
	}
}
