package fixtures

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// BlockHeaderGenerator generates block headers with consistent randomness.
type BlockHeaderGenerator struct {
	randomGen        *RandomGenerator
	identifierGen    *IdentifierGenerator
	signatureGen     *SignatureGenerator
	signerIndicesGen *SignerIndicesGenerator
	quorumCertGen    *QuorumCertificateGenerator
	timeGen          *TimeGenerator
}

type blockHeaderConfig struct {
	height  uint64
	view    uint64
	chainID flow.ChainID
	parent  *flow.Header
	source  []byte
}

// WithHeight returns an option to set the height of the block header.
func (g *BlockHeaderGenerator) WithHeight(height uint64) func(*blockHeaderConfig) {
	return func(config *blockHeaderConfig) {
		config.height = height
	}
}

// WithView returns an option to set the view of the block header.
func (g *BlockHeaderGenerator) WithView(view uint64) func(*blockHeaderConfig) {
	return func(config *blockHeaderConfig) {
		config.view = view
	}
}

// WithChainID returns an option to set the chain ID of the block header.
func (g *BlockHeaderGenerator) WithChainID(chainID flow.ChainID) func(*blockHeaderConfig) {
	return func(config *blockHeaderConfig) {
		config.chainID = chainID
	}
}

// WithParent returns an option to set the parent of the block header.
// Note: if parent is set, values for height, view, and chainID are ignored.
func (g *BlockHeaderGenerator) WithParent(parent *flow.Header) func(*blockHeaderConfig) {
	return func(config *blockHeaderConfig) {
		config.parent = parent
	}
}

// WithParentAndSoR returns an option to set the parent and source of randomness of the block header.
func (g *BlockHeaderGenerator) WithParentAndSoR(parent *flow.Header, source []byte) func(*blockHeaderConfig) {
	return func(config *blockHeaderConfig) {
		config.parent = parent
		config.source = source
	}
}

// Fixture generates a basic block header with random values.
func (g *BlockHeaderGenerator) Fixture(t testing.TB, opts ...func(*blockHeaderConfig)) *flow.Header {
	height := 1 + uint64(g.randomGen.Uint32()) // avoiding edge case of height = 0 (genesis block)
	view := height + uint64(g.randomGen.Intn(1000))

	config := &blockHeaderConfig{
		height:  height,
		view:    view,
		chainID: flow.Emulator,
	}

	for _, opt := range opts {
		opt(config)
	}

	if config.parent != nil {
		// make sure the view and height are valid with respect to the parent
		if config.height <= config.parent.Height {
			if config.height != height {
				// user provided height is invalid
				t.Fatalf("height must be greater than parent height")
			}
			config.height = config.parent.Height + 1
		}
		if config.view <= config.parent.View {
			if config.view != view {
				// user provided view is invalid
				t.Fatalf("view must be greater than parent view")
			}
			config.view = config.parent.View + 1
		}
		return g.fixtureWithParent(t, config)
	}

	if config.height == 0 && config.view == 0 {
		// this is the genesis block
		return g.Genesis(t, opts...)
	}

	if config.height == 0 || config.view == 0 {
		t.Fatalf("height and view must either both be greater than 0 or both be 0 (genesis)")
	}

	// Use minimal fixed values for parent header to avoid disrupting deterministic generation
	// while satisfying validation requirements
	parent, err := flow.NewHeader(flow.UntrustedHeader{
		HeaderBody: flow.HeaderBody{
			ChainID:            config.chainID,
			Height:             config.height - 1,
			View:               config.view - 1,
			ParentID:           g.identifierGen.Fixture(t),
			ParentVoterIndices: g.signerIndicesGen.Fixture(t),
			ParentVoterSigData: g.signatureGen.Fixture(t),
			ProposerID:         g.identifierGen.Fixture(t),
			Timestamp:          uint64(g.timeGen.Fixture(t).UnixMilli()),
		},
		PayloadHash: g.identifierGen.Fixture(t),
	})
	require.NoError(t, err)
	config.parent = parent

	return g.fixtureWithParent(t, config)
}

// Genesis generates a genesis block header.
func (g *BlockHeaderGenerator) Genesis(t testing.TB, opts ...func(*blockHeaderConfig)) *flow.Header {
	config := &blockHeaderConfig{
		chainID: flow.Emulator,
	}

	for _, opt := range opts {
		opt(config)
	}

	headerBody, err := flow.NewRootHeaderBody(
		flow.UntrustedHeaderBody{
			ChainID:   config.chainID,
			ParentID:  flow.ZeroID,
			Height:    0,
			Timestamp: uint64(flow.GenesisTime.UnixMilli()),
			View:      0,
		},
	)
	require.NoError(t, err)

	parent, err := flow.NewHeader(flow.UntrustedHeader{
		HeaderBody: *headerBody,
	})
	require.NoError(t, err)

	return parent
}

// fixtureWithParent generates a block header that is a child of the given parent header.
func (g *BlockHeaderGenerator) fixtureWithParent(t testing.TB, config *blockHeaderConfig) *flow.Header {
	height := config.height
	view := config.view
	parent := config.parent

	var lastViewTC *flow.TimeoutCertificate
	if view != parent.View+1 {
		newestQC := g.quorumCertGen.Fixture(t, g.quorumCertGen.WithView(parent.View))
		lastViewTC = &flow.TimeoutCertificate{
			View:          view - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: g.signerIndicesGen.Fixture(t),
			SigData:       g.signatureGen.Fixture(t),
		}
	}

	// Create the header using the new constructor pattern
	header, err := flow.NewHeader(flow.UntrustedHeader{
		HeaderBody: flow.HeaderBody{
			ChainID:            parent.ChainID,
			ParentID:           parent.ID(),
			Height:             height,
			Timestamp:          uint64(g.timeGen.Fixture(t).UnixMilli()),
			View:               view,
			ParentView:         parent.View,
			ParentVoterIndices: g.signerIndicesGen.Fixture(t),
			ParentVoterSigData: g.quorumCertGen.QCSigDataWithSoR(t, config.source),
			ProposerID:         g.identifierGen.Fixture(t),
			LastViewTC:         lastViewTC,
		},
		PayloadHash: g.identifierGen.Fixture(t),
	})
	require.NoError(t, err)

	return header
}

// List generates a list of block headers.
func (g *BlockHeaderGenerator) List(t testing.TB, n int, opts ...func(*blockHeaderConfig)) []*flow.Header {
	headers := make([]*flow.Header, n)
	for i := range n {
		headers[i] = g.Fixture(t, opts...)
	}
	return headers
}
