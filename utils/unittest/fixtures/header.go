package fixtures

import (
	"math"

	"github.com/onflow/flow-go/model/flow"
)

// Header is the default options factory for [flow.Header] generation.
var Header headerFactory

type headerFactory struct{}

type HeaderOption func(*HeaderGenerator, *flow.Header)

// WithHeight is an option that sets the `Height` of the block header.
func (f headerFactory) WithHeight(height uint64) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.Height = height
	}
}

// WithView is an option that sets the `View` of the block header.
func (f headerFactory) WithView(view uint64) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.View = view
	}
}

// WithChainID is an option that sets the `ChainID` of the block header.
func (f headerFactory) WithChainID(chainID flow.ChainID) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.ChainID = chainID
	}
}

// WithParent is an option that sets the `ParentID`, `ParentView`, and `Height` of the block header based
// on the provided fields. `Height` is set to parent's `Height` + 1.
func (f headerFactory) WithParent(parentID flow.Identifier, parentView uint64, parentHeight uint64) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.ParentID = parentID
		header.ParentView = parentView
		header.Height = parentHeight + 1
	}
}

// WithParentView is an option that sets the `ParentView` of the block header.
func (f headerFactory) WithParentView(view uint64) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.ParentView = view
	}
}

// WithParentHeader is an option that sets the following fields of the block header based on the
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
func (f headerFactory) WithParentHeader(parent *flow.Header) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.View = parent.View + 1
		header.Height = parent.Height + 1
		header.ChainID = parent.ChainID
		header.Timestamp = g.random.Uint64InRange(parent.Timestamp+1, parent.Timestamp+1000)
		header.ParentID = parent.ID()
		header.ParentView = parent.View
	}
}

// WithProposerID is an option that sets the `ProposerID` of the block header.
func (f headerFactory) WithProposerID(proposerID flow.Identifier) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.ProposerID = proposerID
	}
}

// WithLastViewTC is an option that sets the `LastViewTC` of the block header.
func (f headerFactory) WithLastViewTC(lastViewTC *flow.TimeoutCertificate) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.LastViewTC = lastViewTC
	}
}

// WithPayloadHash is an option that sets the `PayloadHash` of the block header.
func (f headerFactory) WithPayloadHash(hash flow.Identifier) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.PayloadHash = hash
	}
}

// WithTimestamp is an option that sets the `Timestamp` of the block header.
func (f headerFactory) WithTimestamp(timestamp uint64) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.Timestamp = timestamp
	}
}

// WithParentVoterIndices is an option that sets the `ParentVoterIndices` of the block header.
func (f headerFactory) WithParentVoterIndices(indices []byte) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.ParentVoterIndices = indices
	}
}

// WithParentVoterSigData is an option that sets the `ParentVoterSigData` of the block header.
func (f headerFactory) WithParentVoterSigData(data []byte) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.ParentVoterSigData = data
	}
}

// WithSourceOfRandomness is an option that sets the `ParentVoterSigData` of the block header based on
// the provided source of randomness.
func (f headerFactory) WithSourceOfRandomness(source []byte) HeaderOption {
	return func(g *HeaderGenerator, header *flow.Header) {
		header.ParentVoterSigData = g.quorumCerts.QCSigDataWithSoR(source)
	}
}

// HeaderGenerator generates block headers with consistent randomness.
type HeaderGenerator struct {
	headerFactory

	random        *RandomGenerator
	identifiers   *IdentifierGenerator
	signatures    *SignatureGenerator
	signerIndices *SignerIndicesGenerator
	quorumCerts   *QuorumCertificateGenerator
	timeGen       *TimeGenerator

	chainID flow.ChainID
}

func NewBlockHeaderGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	signatures *SignatureGenerator,
	signerIndices *SignerIndicesGenerator,
	quorumCerts *QuorumCertificateGenerator,
	timeGen *TimeGenerator,
	chainID flow.ChainID,
) *HeaderGenerator {
	return &HeaderGenerator{
		random:        random,
		identifiers:   identifiers,
		signatures:    signatures,
		signerIndices: signerIndices,
		quorumCerts:   quorumCerts,
		timeGen:       timeGen,
		chainID:       chainID,
	}
}

// Fixture generates a [flow.Header] with random data based on the provided options.
func (g *HeaderGenerator) Fixture(opts ...HeaderOption) *flow.Header {
	height := g.random.Uint64InRange(1, math.MaxUint32) // avoiding edge case of height = 0 (genesis block)
	view := g.random.Uint64InRange(height, height+1000)

	header := &flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID:            g.chainID,
			ParentID:           g.identifiers.Fixture(),
			Height:             height,
			Timestamp:          uint64(g.timeGen.Fixture().UnixMilli()),
			View:               view,
			ParentView:         view - 1,
			ParentVoterIndices: g.signerIndices.Fixture(),
			ParentVoterSigData: g.signatures.Fixture(),
			ProposerID:         g.identifiers.Fixture(),
			LastViewTC:         nil, // default no TC
		},
		PayloadHash: g.identifiers.Fixture(),
	}

	for _, opt := range opts {
		opt(g, header)
	}

	if header.View != header.ParentView+1 && header.LastViewTC == nil {
		newestQC := g.quorumCerts.Fixture(QuorumCertificate.WithView(header.ParentView))
		header.LastViewTC = &flow.TimeoutCertificate{
			View:          view - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: g.signerIndices.Fixture(),
			SigData:       g.signatures.Fixture(),
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
// The only option that's used is the chainID.
func (g *HeaderGenerator) Genesis(opts ...HeaderOption) *flow.Header {
	config := &flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID: g.chainID,
		},
	}

	// allow overriding the chainID
	for _, opt := range opts {
		opt(g, config)
	}

	header, err := flow.NewRootHeader(flow.UntrustedHeader{
		HeaderBody: flow.HeaderBody{
			ChainID:   config.ChainID,
			ParentID:  flow.ZeroID,
			Height:    0,
			Timestamp: uint64(flow.GenesisTime.UnixMilli()),
			View:      0,
		},
		PayloadHash: flow.NewEmptyPayload().Hash(),
	})
	NoError(err)
	return header
}

// List generates a chain of [flow.Header]. The first header is generated with the given options,
// and the subsequent headers are generated using the previous header as the parent.
func (g *HeaderGenerator) List(n int, opts ...HeaderOption) []*flow.Header {
	headers := make([]*flow.Header, 0, n)
	headers = append(headers, g.Fixture(opts...))

	for i := 1; i < n; i++ {
		// give a 50% chance that the view is not ParentView + 1
		view := headers[i-1].View + 1
		if g.random.Bool() {
			view += g.random.Uint64InRange(1, 10)
		}

		headers = append(headers, g.Fixture(
			Header.WithParentHeader(headers[i-1]),
			Header.WithView(view),
		))
	}
	return headers
}
