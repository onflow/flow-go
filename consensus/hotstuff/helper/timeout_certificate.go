package helper

import (
	"math/rand"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func MakeTC(options ...func(*flow.TimeoutCertificate)) *flow.TimeoutCertificate {
	qc := MakeQC()
	tc := flow.TimeoutCertificate{
		View:          rand.Uint64(),
		TOHighestQC:   qc,
		TOHighQCViews: []uint64{qc.View},
		SignerIDs:     unittest.IdentityListFixture(7).NodeIDs(),
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
		for _, view := range tc.TOHighQCViews {
			if view == qc.View {
				return
			}
		}
		tc.TOHighQCViews = append(tc.TOHighQCViews, qc.View)
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
