package unittest

import (
	"github.com/onflow/flow-go/model/flow"
)

var Block blockFactory

type blockFactory struct{}

// BlockFixture initializes and returns a new *flow.Block instance.
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
