package unittest

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

var Block blockFactory

type blockFactory struct{}

// BlockFixture initializes and returns a new *flow.UnsignedBlock instance.
func BlockFixture(opts ...func(*flow.UnsignedBlock)) *flow.UnsignedBlock {
	header := BlockHeaderFixture()
	block := BlockWithParentFixture(header)
	for _, opt := range opts {
		opt(block)
	}
	return block
}

func (f *blockFactory) WithParent(parentID flow.Identifier, parentView uint64, parentHeight uint64) func(*flow.UnsignedBlock) {
	return func(block *flow.UnsignedBlock) {
		block.ParentID = parentID
		block.ParentView = parentView
		block.Height = parentHeight + 1
	}
}

func (f *blockFactory) WithView(view uint64) func(*flow.UnsignedBlock) {
	return func(block *flow.UnsignedBlock) {
		block.View = view
	}
}

func (f *blockFactory) WithParentView(view uint64) func(*flow.UnsignedBlock) {
	return func(block *flow.UnsignedBlock) {
		block.ParentView = view
	}
}

func (f *blockFactory) WithHeight(height uint64) func(*flow.UnsignedBlock) {
	return func(block *flow.UnsignedBlock) {
		block.Height = height
	}
}

func (f *blockFactory) WithPayload(payload flow.Payload) func(*flow.UnsignedBlock) {
	return func(b *flow.UnsignedBlock) {
		b.Payload = payload
	}
}

func (f *blockFactory) WithProposerID(proposerID flow.Identifier) func(*flow.UnsignedBlock) {
	return func(b *flow.UnsignedBlock) {
		b.ProposerID = proposerID
	}
}

func (f *blockFactory) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*flow.UnsignedBlock) {
	return func(block *flow.UnsignedBlock) {
		block.LastViewTC = lastViewTC
	}
}

func (f *blockFactory) Genesis(chainID flow.ChainID) *flow.UnsignedBlock {
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
		flow.UntrustedUnsignedBlock{
			HeaderBody: *headerBody,
			Payload:    payload,
		},
	)
	if err != nil {
		panic(fmt.Errorf("failed to create root block: %w", err))
	}

	return block
}
