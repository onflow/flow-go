package fixtures

import (
	"testing"

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
		config.parent = &flow.Header{
			ChainID:  config.chainID,
			Height:   config.height - 1,
			View:     config.view - 1,
			ParentID: g.identifierGen.Fixture(t),
		}
	}

	return g.fixtureWithParent(t, config.parent, config.source)
}

// fixtureWithParent generates a block header that is a child of the given parent header.
func (g *BlockHeaderGenerator) fixtureWithParent(t testing.TB, parent *flow.Header, source []byte) *flow.Header {
	height := parent.Height + 1
	view := parent.View + 1 + uint64(g.randomGen.Intn(10)) // Intn returns [0, n)

	var lastViewTC *flow.TimeoutCertificate
	if view != parent.View+1 {
		newestQC := g.quorumCertGen.Fixture(t, g.quorumCertGen.WithView(parent.View))
		lastViewTC = &flow.TimeoutCertificate{
			View:          view - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: g.signerIndicesGen.Fixture(t, 4),
			SigData:       g.signatureGen.Fixture(t),
		}
	}

	return &flow.Header{
		ChainID:            parent.ChainID,
		ParentID:           parent.ID(),
		Height:             height,
		PayloadHash:        g.identifierGen.Fixture(t),
		Timestamp:          g.timeGen.Fixture(t),
		View:               view,
		ParentView:         parent.View,
		ParentVoterIndices: g.signerIndicesGen.Fixture(t, 4),
		ParentVoterSigData: g.quorumCertGen.QCSigDataWithSoR(t, source),
		ProposerID:         g.identifierGen.Fixture(t),
		ProposerSigData:    g.signatureGen.Fixture(t),
		LastViewTC:         lastViewTC,
	}
}
