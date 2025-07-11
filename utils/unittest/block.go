package unittest

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const GenesisProtocolStateIDHex = "4745d52bae83d7e106865a57617a2d3310403fce45ff21a8786aa47c5bd047c5"
const GenesisParentIDHex = "037e61db6167dd01dd0dcbaf6a5c3656e87083672e9d8862f4e1af9d34bc98e7"

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

func (f *blockFactory) WithParentView(view uint64) func(*flow.Block) {
	return func(block *flow.Block) {
		block.Header.ParentView = view
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

func (f *blockFactory) Genesis(chainID flow.ChainID) (*flow.Block, error) {
	// must be const, so that the pre-calculated state commitments (GenesisStateCommitmentHex and others) always stay the same for tests
	protocolStateID, err := flow.HexStringToIdentifier(GenesisProtocolStateIDHex)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ProtocolStateID: %w", err)
	}

	parentID, err := flow.HexStringToIdentifier(GenesisParentIDHex)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ParentID: %w", err)
	}

	// create the raw content for the genesis block
	payload := flow.Payload{
		ProtocolStateID: protocolStateID,
	}

	// create the headerBody
	headerBody, err := flow.NewRootHeaderBody(
		flow.UntrustedHeaderBody{
			ChainID:   chainID,
			ParentID:  parentID,
			Height:    0,
			Timestamp: flow.GenesisTime,
			View:      0,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root header body: %w", err)
	}

	// combine to block
	block, err := flow.NewRootBlock(
		flow.UntrustedBlock{
			Header:  *headerBody,
			Payload: payload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root block: %w", err)
	}
	return block, nil
}
