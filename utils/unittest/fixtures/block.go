package fixtures

import "github.com/onflow/flow-go/model/flow"

// Block is the default options factory for [flow.Block] generation.
var Block blockFactory

type blockFactory struct{}

type BlockOption func(*BlockGenerator, *flow.Block)

// WithHeight is an option that sets the `Height` of the block's header.
func (f blockFactory) WithHeight(height uint64) BlockOption {
	return func(g *BlockGenerator, header *flow.Block) {
		header.Height = height
	}
}

// WithView is an option that sets the `View` of the block's header.
func (f blockFactory) WithView(view uint64) BlockOption {
	return func(g *BlockGenerator, header *flow.Block) {
		header.View = view
	}
}

// WithChainID is an option that sets the `ChainID` of the block's header.
func (f blockFactory) WithChainID(chainID flow.ChainID) BlockOption {
	return func(g *BlockGenerator, header *flow.Block) {
		header.ChainID = chainID
	}
}

// WithParent is an option that sets the `ParentID`, `ParentView`, and `Height` of the block's header based
// on the provided fields. `Height` is set to parent's `Height` + 1.
func (f blockFactory) WithParent(parentID flow.Identifier, parentView uint64, parentHeight uint64) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.ParentID = parentID
		block.ParentView = parentView
		block.Height = parentHeight + 1
	}
}

// WithParentView is an option that sets the `ParentView` of the block's header.
func (f blockFactory) WithParentView(view uint64) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.ParentView = view
	}
}

// WithParentHeader is an option that sets the the following fields of the block's header based on the
// provided parent header:
// - `View`
// - `Height`
// - `ChainID`
// - `Timestamp`
// - `ParentID`
// - `ParentView`
//
// If you want a specific value for any of these fields, you should add the appropriate option
// after this option.
func (f blockFactory) WithParentHeader(parent *flow.Header) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.View = parent.View + 1
		block.Height = parent.Height + 1
		block.ChainID = parent.ChainID
		block.Timestamp = g.random.Uint64InRange(parent.Timestamp+1, parent.Timestamp+1000)
		block.ParentID = parent.ID()
		block.ParentView = parent.View
	}
}

// WithProposerID is an option that sets the `ProposerID` of the block's header.
func (f blockFactory) WithProposerID(proposerID flow.Identifier) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.ProposerID = proposerID
	}
}

// WithLastViewTC is an option that sets the `LastViewTC` of the block's header.
func (f blockFactory) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.LastViewTC = lastViewTC
	}
}

// WithTimestamp is an option that sets the `Timestamp` of the block's header.
func (f blockFactory) WithTimestamp(timestamp uint64) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.Timestamp = timestamp
	}
}

// WithParentVoterIndices is an option that sets the `ParentVoterIndices` of the block's header.
func (f blockFactory) WithParentVoterIndices(indices []byte) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.ParentVoterIndices = indices
	}
}

// WithParentVoterSigData is an option that sets the `ParentVoterSigData` of the block's header.
func (f blockFactory) WithParentVoterSigData(data []byte) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.ParentVoterSigData = data
	}
}

// WithHeaderBody is an option that sets the `HeaderBody` of the block.
func (f blockFactory) WithHeaderBody(headerBody flow.HeaderBody) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.HeaderBody = headerBody
	}
}

// WithPayload is an option that sets the `Payload` of the block.
func (f blockFactory) WithPayload(payload flow.Payload) BlockOption {
	return func(g *BlockGenerator, block *flow.Block) {
		block.Payload = payload
	}
}

// BlockGenerator generates blocks with consistent randomness.
type BlockGenerator struct {
	blockFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
	headers     *HeaderGenerator
	payloads    *PayloadGenerator

	chainID flow.ChainID
}

func NewBlockGenerator(
	identifiers *IdentifierGenerator,
	headers *HeaderGenerator,
	payloads *PayloadGenerator,
	chainID flow.ChainID,
) *BlockGenerator {
	return &BlockGenerator{
		identifiers: identifiers,
		headers:     headers,
		payloads:    payloads,
		chainID:     chainID,
	}
}

// Fixture generates a [flow.Block] with random data based on the provided options.
func (g *BlockGenerator) Fixture(opts ...BlockOption) *flow.Block {
	header := g.headers.Fixture(Header.WithChainID(g.chainID))
	payload := g.payloads.Fixture()

	block := &flow.Block{
		HeaderBody: header.HeaderBody,
		Payload:    *payload,
	}

	for _, opt := range opts {
		opt(g, block)
	}

	return block
}

// List generates a chain of [flow.Block] objects. The first block is generated with the given options,
// and the subsequent blocks are generated using only the WithParentHeader option, specifying the
// previous block as the parent.
func (g *BlockGenerator) List(n int, opts ...BlockOption) []*flow.Block {
	blocks := make([]*flow.Block, 0, n)
	blocks = append(blocks, g.Fixture(opts...))

	for i := 1; i < n; i++ {
		parent := blocks[i-1].ToHeader()
		blocks = append(blocks, g.Fixture(Block.WithParentHeader(parent)))
	}
	return blocks
}

// Genesis instantiates a genesis block. This block has view and height equal to zero. However,
// conceptually spork root blocks are functionally equivalent to genesis blocks. We have decided that
// in the long term, the protocol must support spork root blocks with height _and_ view larger than zero.
// The only options that are used are the chainID and the protocol state ID.
func (g *BlockGenerator) Genesis(opts ...BlockOption) *flow.Block {
	config := &flow.Block{
		HeaderBody: flow.HeaderBody{
			ChainID: g.chainID,
		},
		Payload: flow.Payload{
			ProtocolStateID: g.identifiers.Fixture(),
		},
	}

	// allow overriding the chainID and the protocol state ID
	for _, opt := range opts {
		opt(g, config)
	}

	header := g.headers.Genesis(Header.WithChainID(config.ChainID))
	block, err := flow.NewRootBlock(flow.UntrustedBlock{
		HeaderBody: header.HeaderBody,
		Payload: flow.Payload{
			ProtocolStateID: config.Payload.ProtocolStateID,
		},
	})
	NoError(err)

	return block
}
