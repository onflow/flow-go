package unittest

import (
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

var Block blockFactory

type blockFactory struct{}

// BlockFixture initializes and returns a new flow.Block instance.
func BlockFixture(opts ...func(*flow.Block)) *flow.Block {
	header := BlockHeaderFixture()
	block := BlockWithParentFixture(header)
	for _, opt := range opts {
		opt(block)
	}
	return block
}

func (f *blockFactory) WithParent(parentID flow.Identifier, parentView uint64, parentHeight uint64) func(*flow.Block) {
	return func(block *flow.Block) {
		block.Header.ParentID = parentID
		block.Header.ParentView = parentView
		block.Header.Height = parentHeight + 1
		block.Header.View = parentView + 1 + uint64(rand.Intn(10)) // Intn returns [0, n)
		block.Header.LastViewTC = f.lastViewTCFixture(block.Header.View, block.Header.ParentView)
	}
}

func (f *blockFactory) WithView(view uint64) func(*flow.Block) {
	return func(block *flow.Block) {
		block.Header.View = view
	}
}

func (f *blockFactory) WithHeight(height uint64) func(*flow.Block) {
	return func(block *flow.Block) {
		block.Header.Height = height
	}
}

func (f *blockFactory) WithPayload(payload flow.Payload) func(*flow.Block) {
	return func(b *flow.Block) {
		b.Payload = payload
	}
}

func (f *blockFactory) WithProposerID(proposerID flow.Identifier) func(*flow.Block) {
	return func(b *flow.Block) {
		b.Header.ProposerID = proposerID
	}
}

func (f *blockFactory) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*flow.Block) {
	return func(block *flow.Block) {
		block.Header.LastViewTC = lastViewTC
	}
}

func (f *blockFactory) lastViewTCFixture(view uint64, parentView uint64) *flow.TimeoutCertificate {
	var lastViewTC *flow.TimeoutCertificate
	if view != parentView+1 {
		newestQC := QuorumCertificateFixture(func(qc *flow.QuorumCertificate) {
			qc.View = parentView
		})
		lastViewTC = &flow.TimeoutCertificate{
			View:          view - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: SignerIndicesFixture(4),
			SigData:       SignatureFixture(),
		}
	}
	return lastViewTC
}
