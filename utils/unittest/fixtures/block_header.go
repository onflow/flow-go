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

	if config.parent == nil {
		parent, err := flow.NewHeader(flow.UntrustedHeader{
			HeaderBody: flow.HeaderBody{
				ChainID:  config.chainID,
				Height:   config.height - 1,
				View:     config.view - 1,
				ParentID: g.identifierGen.Fixture(t),
			},
			PayloadHash: g.identifierGen.Fixture(t),
		})
		require.NoError(t, err)
		config.parent = parent
	}

	return g.fixtureWithParent(t, config)
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
