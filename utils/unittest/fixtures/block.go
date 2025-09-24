package fixtures

import "github.com/onflow/flow-go/model/flow"

// Block is the default options factory for [flow.Block] generation.
var Block blockFactory

type blockFactory struct{}

type BlockOption func(*BlockGenerator, *blockConfig)

type blockConfig struct {
	headerOpts []HeaderOption

	headerBody *flow.HeaderBody
	payload    *flow.Payload
}

// WithHeight is an option that sets the `Height` of the block's header.
func (f blockFactory) WithHeight(height uint64) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts, Header.WithHeight(height))

	}
}

// WithView is an option that sets the `View` of the block's header.
func (f blockFactory) WithView(view uint64) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithView(view),
		)
	}
}

// WithChainID is an option that sets the `ChainID` of the block's header.
func (f blockFactory) WithChainID(chainID flow.ChainID) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithChainID(chainID),
		)
	}
}

// WithParent is an option that sets the `ParentID`, `ParentView`, and `Height` of the block's header based
// on the provided fields. `Height` is set to parent's `Height` + 1.
func (f blockFactory) WithParent(parentID flow.Identifier, parentView uint64, parentHeight uint64) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithParent(parentID, parentView, parentHeight),
		)
	}
}

// WithParentView is an option that sets the `ParentView` of the block's header.
func (f blockFactory) WithParentView(view uint64) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithParentView(view),
		)
	}
}

// WithParentHeader is an option that sets the following fields of the block's header based on the
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
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithParentHeader(parent),
		)
	}
}

// WithProposerID is an option that sets the `ProposerID` of the block's header.
func (f blockFactory) WithProposerID(proposerID flow.Identifier) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithProposerID(proposerID),
		)
	}
}

// WithLastViewTC is an option that sets the `LastViewTC` of the block's header.
func (f blockFactory) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithLastViewTC(lastViewTC),
		)
	}
}

// WithTimestamp is an option that sets the `Timestamp` of the block's header.
func (f blockFactory) WithTimestamp(timestamp uint64) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithTimestamp(timestamp),
		)
	}
}

// WithParentVoterIndices is an option that sets the `ParentVoterIndices` of the block's header.
func (f blockFactory) WithParentVoterIndices(indices []byte) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithParentVoterIndices(indices),
		)
	}
}

// WithParentVoterSigData is an option that sets the `ParentVoterSigData` of the block's header.
func (f blockFactory) WithParentVoterSigData(data []byte) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerOpts = append(config.headerOpts,
			Header.WithParentVoterSigData(data),
		)
	}
}

// WithHeaderBody is an option that sets the `HeaderBody` of the block.
func (f blockFactory) WithHeaderBody(headerBody *flow.HeaderBody) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.headerBody = headerBody
	}
}

// WithPayload is an option that sets the `Payload` of the block.
func (f blockFactory) WithPayload(payload *flow.Payload) BlockOption {
	return func(g *BlockGenerator, config *blockConfig) {
		config.payload = payload
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
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	headers *HeaderGenerator,
	payloads *PayloadGenerator,
	chainID flow.ChainID,
) *BlockGenerator {
	return &BlockGenerator{
		random:      random,
		identifiers: identifiers,
		headers:     headers,
		payloads:    payloads,
		chainID:     chainID,
	}
}

// Fixture generates a [flow.Block] with random data based on the provided options.
func (g *BlockGenerator) Fixture(opts ...BlockOption) *flow.Block {
	config := &blockConfig{
		headerOpts: []HeaderOption{Header.WithChainID(g.chainID)},
	}

	for _, opt := range opts {
		opt(g, config)
	}

	if config.headerBody == nil {
		header := g.headers.Fixture(config.headerOpts...)
		config.headerBody = &header.HeaderBody
	}
	if config.payload == nil {
		config.payload = g.payloads.Fixture()
	}

	return &flow.Block{
		HeaderBody: *config.headerBody,
		Payload:    *config.payload,
	}
}

// List generates a chain of [flow.Block] objects. The first block is generated with the given options,
// and the subsequent blocks are generated using only the WithParentHeader option, specifying the
// previous block as the parent.
func (g *BlockGenerator) List(n int, opts ...BlockOption) []*flow.Block {
	blocks := make([]*flow.Block, 0, n)
	blocks = append(blocks, g.Fixture(opts...))

	for i := 1; i < n; i++ {
		// give a 50% chance that the view is not ParentView + 1
		view := blocks[i-1].View + 1
		if g.random.Bool() {
			view += g.random.Uint64InRange(1, 10)
		}

		parent := blocks[i-1].ToHeader()
		blocks = append(blocks, g.Fixture(
			Block.WithParentHeader(parent),
			Block.WithView(view),
		))
	}
	return blocks
}

// Genesis instantiates a genesis block. This block has view and height equal to zero. However,
// conceptually spork root blocks are functionally equivalent to genesis blocks. We have decided that
// in the long term, the protocol must support spork root blocks with height _and_ view larger than zero.
// The only options that are used are the chainID and the protocol state ID.
func (g *BlockGenerator) Genesis(opts ...BlockOption) *flow.Block {
	config := &blockConfig{
		headerOpts: []HeaderOption{Header.WithChainID(g.chainID)},
		payload: &flow.Payload{
			ProtocolStateID: g.identifiers.Fixture(),
		},
	}

	// allow overriding the chainID and the protocol state ID
	for _, opt := range opts {
		opt(g, config)
	}

	header := g.headers.Genesis(config.headerOpts...)
	if config.payload == nil {
		config.payload = &flow.Payload{
			ProtocolStateID: g.identifiers.Fixture(),
		}
	}

	block, err := flow.NewRootBlock(flow.UntrustedBlock{
		HeaderBody: header.HeaderBody,
		Payload:    *config.payload,
	})
	NoError(err)

	return block
}
