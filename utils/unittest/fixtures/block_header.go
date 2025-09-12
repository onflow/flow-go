package fixtures

import (
	"math"

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

	chainID flow.ChainID
}

func NewBlockHeaderGenerator(
	randomGen *RandomGenerator,
	identifierGen *IdentifierGenerator,
	signatureGen *SignatureGenerator,
	signerIndicesGen *SignerIndicesGenerator,
	quorumCertGen *QuorumCertificateGenerator,
	timeGen *TimeGenerator,
	chainID flow.ChainID,
) *BlockHeaderGenerator {
	return &BlockHeaderGenerator{
		randomGen:        randomGen,
		identifierGen:    identifierGen,
		signatureGen:     signatureGen,
		signerIndicesGen: signerIndicesGen,
		quorumCertGen:    quorumCertGen,
		timeGen:          timeGen,
		chainID:          chainID,
	}
}

// WithHeight is an option that sets the height of the block header.
func (g *BlockHeaderGenerator) WithHeight(height uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.Height = height
	}
}

// WithView is an option that sets the view of the block header.
func (g *BlockHeaderGenerator) WithView(view uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.View = view
	}
}

// WithChainID is an option that sets the chain ID of the block header.
func (g *BlockHeaderGenerator) WithChainID(chainID flow.ChainID) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ChainID = chainID
	}
}

// WithParent is an option that sets the ParentID, ParentView, and Height of the block header based
// on the provided fields. Height is set to parent's Height + 1.
func (g *BlockHeaderGenerator) WithParent(parentID flow.Identifier, parentView uint64, parentHeight uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ParentID = parentID
		header.ParentView = parentView
		header.Height = parentHeight + 1
	}
}

// WithParentView is an option that sets the ParentView of the block header.
func (g *BlockHeaderGenerator) WithParentView(view uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ParentView = view
	}
}

// WithParentHeader is an option that sets the the following fields of the block header based on the
// provided parent header:
// - View
// - Height
// - ChainID
// - Timestamp
// - ParentID
// - ParentView
//
// If you want a specific value for any of these fields, you should add the appropriate option
// after this option.
func (g *BlockHeaderGenerator) WithParentHeader(parent *flow.Header) func(*flow.Header) {
	return func(header *flow.Header) {
		header.View = header.ParentView + 1
		header.Height = parent.Height + 1
		header.ChainID = parent.ChainID
		header.Timestamp = g.randomGen.Uint64InRange(parent.Timestamp+1, parent.Timestamp+1000)
		header.ParentID = parent.ID()
		header.ParentView = parent.View
	}
}

// WithProposerID is an option that sets the ProposerID of the block header.
func (g *BlockHeaderGenerator) WithProposerID(proposerID flow.Identifier) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ProposerID = proposerID
	}
}

// WithLastViewTC is an option that sets the LastViewTC of the block header.
func (g *BlockHeaderGenerator) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) func(*flow.Header) {
	return func(header *flow.Header) {
		header.LastViewTC = lastViewTC
	}
}

// WithPayloadHash is an option that sets the PayloadHash of the block header.
func (g *BlockHeaderGenerator) WithPayloadHash(hash flow.Identifier) func(*flow.Header) {
	return func(header *flow.Header) {
		header.PayloadHash = hash
	}
}

// WithTimestamp is an option that sets the Timestamp of the block header.
func (g *BlockHeaderGenerator) WithTimestamp(timestamp uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.Timestamp = timestamp
	}
}

// WithSourceOfRandomness is an option that sets the ParentVoterSigData of the block header based on
// the provided source of randomness.
func (g *BlockHeaderGenerator) WithSourceOfRandomness(source []byte) func(*flow.Header) {
	return func(header *flow.Header) {
		header.ParentVoterSigData = g.quorumCertGen.QCSigDataWithSoR(source)
	}
}

// Fixture generates a [flow.Header] with random data based on the provided options.
func (g *BlockHeaderGenerator) Fixture(opts ...func(*flow.Header)) *flow.Header {
	height := g.randomGen.Uint64InRange(1, math.MaxUint32) // avoiding edge case of height = 0 (genesis block)
	view := g.randomGen.Uint64InRange(height, height+1000)

	header := &flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID:            g.chainID,
			ParentID:           g.identifierGen.Fixture(),
			Height:             height,
			Timestamp:          uint64(g.timeGen.Fixture().UnixMilli()),
			View:               view,
			ParentView:         view - 1,
			ParentVoterIndices: g.signerIndicesGen.Fixture(),
			ParentVoterSigData: g.signatureGen.Fixture(),
			ProposerID:         g.identifierGen.Fixture(),
			LastViewTC:         nil, // default no TC
		},
		PayloadHash: g.identifierGen.Fixture(),
	}

	for _, opt := range opts {
		opt(header)
	}

	if header.View != header.ParentView+1 && header.LastViewTC == nil {
		newestQC := g.quorumCertGen.Fixture(g.quorumCertGen.WithView(header.ParentView))
		header.LastViewTC = &flow.TimeoutCertificate{
			View:          view - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: g.signerIndicesGen.Fixture(),
			SigData:       g.signatureGen.Fixture(),
		}
	}

	// View must be strictly greater than ParentView. Since we are generating default values for each
	// and allowing the caller to independently update them, we need to do some extra bookkeeping to
	// ensure that the values remain consistent after applying the options. Since the values start
	// in a consistent state, if they are now inconsistent, there are 3 possible cases:
	// 1. View was updated and ParentView was not
	// 	 -> adjust ParentView to align with the user set value
	// 2. View was not updated and ParentView was updated
	// 	 -> adjust View to align with the user set value
	// 3. Both were updated
	// 	 -> do nothing since the user specifically configured it this way
	if header.View <= header.ParentView {
		if header.View != view && header.ParentView == view-1 { // case 1
			header.ParentView = header.View - 1
		}
		if header.View == view && header.ParentView != view-1 { // case 2
			header.View = header.ParentView + 1
		}
	}

	// sanity checks
	Assertf(header.View > header.ParentView,
		"view must be greater than or equal to parent view: %d > %d", header.View, header.ParentView)

	Assertf(header.Height > 0 || (header.Height == 0 && header.View == 0),
		"height and view must either both be greater than 0 or both be 0 (genesis): (height: %d, view: %d)",
		header.Height, header.View)

	Assertf(header.LastViewTC != nil || header.View == header.ParentView+1,
		"last view TC must be present if view is not equal to parent view + 1: (view: %d, parent view: %d)",
		header.View, header.ParentView)

	return header
}

// Genesis instantiates a genesis block header. This block has view and height equal to zero. However,
// conceptually spork root blocks are functionally equivalent to genesis blocks. We have decided that
// in the long term, the protocol must support spork root blocks with height _and_ view larger than zero.
func (g *BlockHeaderGenerator) Genesis(opts ...func(*flow.Header)) *flow.Header {
	header := &flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID: flow.Emulator,
		},
	}

	for _, opt := range opts {
		opt(header)
	}

	return &flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID:   header.ChainID,
			ParentID:  flow.ZeroID,
			Height:    0,
			Timestamp: uint64(flow.GenesisTime.UnixMilli()),
			View:      0,
		},
	}
}

// List generates a chain of block headers. The first block is generated with the given options,
// and the subsequent blocks are generated using the previous block as the parent.
func (g *BlockHeaderGenerator) List(n int, opts ...func(*flow.Header)) []*flow.Header {
	headers := make([]*flow.Header, 0, n)
	headers = append(headers, g.Fixture(opts...))

	for i := 1; i < n; i++ {
		headers = append(headers, g.Fixture(g.WithParentHeader(headers[i-1])))
	}
	return headers
}
