package unittest

import (
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

var Header HeaderFactory

type HeaderFactory struct{}

func (f HeaderFactory) Fixture(options ...func(*flow.Header)) flow.Header {
	height := uint64(rand.Uint32())
	view := height + uint64(rand.Intn(1000))
	header := BlockHeaderWithParentFixture(&flow.Header{
		ChainID:  flow.Emulator,
		ParentID: IdentifierFixture(),
		Height:   height,
		View:     view,
	})

	for _, option := range options {
		option(&header)
	}

	return header
}

var Block BlockFactory

type BlockFactory struct{}

func (f BlockFactory) Fixture(options ...func(*flow.Block)) flow.Block {
	header := BlockHeaderFixture()
	block := BlockWithParentFixture(&header)

	for _, option := range options {
		option(&block)
	}

	return block
}

func (f BlockFactory) WithParent(parent *flow.Header) func(*flow.Block) {
	payload := PayloadFixture(WithoutSeals)
	header := BlockHeaderWithParentFixture(parent)
	header.PayloadHash = payload.Hash()
	return flow.Block{
		Header:  &header,
		Payload: payload,
	}
}

func (f BlockFactory) Fixtures(number int) []*flow.Block {
	blocks := make([]*flow.Block, 0, number)
	for ; number > 0; number-- {
		block := BlockFixture()
		blocks = append(blocks, &block)
	}
	return blocks
}

var Payload PayloadFactory

type PayloadFactory struct{}

func (f PayloadFactory) Fixture(options ...func(*flow.Payload)) *flow.Payload {
	payload := flow.Payload{
		Guarantees: CollectionGuaranteesFixture(16),
		Seals:      Seal.Fixtures(16),
	}
	for _, option := range options {
		option(&payload)
	}
	return &payload
}

func (f PayloadFactory) WithoutSeals() func(*flow.Payload) {
	return func(payload *flow.Payload) {
		payload.Seals = nil
	}
}
