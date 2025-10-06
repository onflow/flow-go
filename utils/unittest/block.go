package unittest

import (
	"fmt"

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
		block.ParentID = parentID
		block.ParentView = parentView
		block.Height = parentHeight + 1
		block.View = parentView + 1
	}
}

func (f *blockFactory) WithView(view uint64) func(*flow.Block) {
	return func(block *flow.Block) {
		block.View = view
	}
}

func (f *blockFactory) WithParentView(view uint64) func(*flow.Block) {
	return func(block *flow.Block) {
		block.ParentView = view
	}
}

func (f *blockFactory) WithHeight(height uint64) func(*flow.Block) {
	return func(block *flow.Block) {
		block.Height = height
	}
}

func (f *blockFactory) WithPayload(payload flow.Payload) func(*flow.Block) {
	return func(b *flow.Block) {
		b.Payload = payload
	}
}

func (f *blockFactory) WithProposerID(proposerID flow.Identifier) func(*flow.Block) {
	return func(b *flow.Block) {
		b.ProposerID = proposerID
	}
}

func (f *blockFactory) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*flow.Block) {
	return func(block *flow.Block) {
		block.LastViewTC = lastViewTC
	}
}

func (f *blockFactory) Genesis(chainID flow.ChainID) *flow.Block {
	// create the raw content for the genesis block
	payload := flow.Payload{
		ProtocolStateID: IdentifierFixture(),
	}

	// create the headerBody
	headerBody, err := flow.NewRootHeaderBody(
		flow.UntrustedHeaderBody{
			ChainID:   chainID,
			ParentID:  flow.ZeroID,
			Height:    0,
			Timestamp: uint64(flow.GenesisTime.UnixMilli()),
			View:      0,
		},
	)
	if err != nil {
		panic(fmt.Errorf("failed to create root header body: %w", err))
	}

	// combine to block
	block, err := flow.NewRootBlock(
		flow.UntrustedBlock{
			HeaderBody: *headerBody,
			Payload:    payload,
		},
	)
	if err != nil {
		panic(fmt.Errorf("failed to create root block: %w", err))
	}

	return block
}
