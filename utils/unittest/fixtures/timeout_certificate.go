package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/slices"
)

// TimeoutCertificate is the default options factory for [flow.TimeoutCertificate] generation.
var TimeoutCertificate timeoutCertificateFactory

type timeoutCertificateFactory struct{}

type TimeoutCertificateOption func(*TimeoutCertificateGenerator, *flow.TimeoutCertificate)

// WithView is an option that sets the `View` of the timeout certificate.
func (f timeoutCertificateFactory) WithView(view uint64) TimeoutCertificateOption {
	return func(g *TimeoutCertificateGenerator, tc *flow.TimeoutCertificate) {
		tc.View = view
	}
}

// WithNewestQC is an option that sets the `NewestQC` of the timeout certificate.
func (f timeoutCertificateFactory) WithNewestQC(qc *flow.QuorumCertificate) TimeoutCertificateOption {
	return func(g *TimeoutCertificateGenerator, tc *flow.TimeoutCertificate) {
		tc.NewestQC = qc
	}
}

// WithNewestQCViews is an option that sets the `NewestQCViews` of the timeout certificate.
func (f timeoutCertificateFactory) WithNewestQCViews(views []uint64) TimeoutCertificateOption {
	return func(g *TimeoutCertificateGenerator, tc *flow.TimeoutCertificate) {
		tc.NewestQCViews = views
	}
}

// WithSignerIndices is an option that sets the `SignerIndices` of the timeout certificate.
func (f timeoutCertificateFactory) WithSignerIndices(indices []byte) TimeoutCertificateOption {
	return func(g *TimeoutCertificateGenerator, tc *flow.TimeoutCertificate) {
		tc.SignerIndices = indices
	}
}

// WithSigData is an option that sets the `SigData` of the timeout certificate.
func (f timeoutCertificateFactory) WithSigData(sigData []byte) TimeoutCertificateOption {
	return func(g *TimeoutCertificateGenerator, tc *flow.TimeoutCertificate) {
		tc.SigData = sigData
	}
}

// TimeoutCertificateGenerator generates timeout certificates with consistent randomness.
type TimeoutCertificateGenerator struct {
	timeoutCertificateFactory

	random        *RandomGenerator
	quorumCerts   *QuorumCertificateGenerator
	signatures    *SignatureGenerator
	signerIndices *SignerIndicesGenerator
}

// NewTimeoutCertificateGenerator creates a new TimeoutCertificateGenerator.
func NewTimeoutCertificateGenerator(
	random *RandomGenerator,
	quorumCerts *QuorumCertificateGenerator,
	signatures *SignatureGenerator,
	signerIndices *SignerIndicesGenerator,
) *TimeoutCertificateGenerator {
	return &TimeoutCertificateGenerator{
		random:        random,
		quorumCerts:   quorumCerts,
		signatures:    signatures,
		signerIndices: signerIndices,
	}
}

// Fixture generates a [flow.TimeoutCertificate] with random data based on the provided options.
func (g *TimeoutCertificateGenerator) Fixture(opts ...TimeoutCertificateOption) *flow.TimeoutCertificate {
	// skip view 0 to avoid edge case where TC view is 0 and QC view is 0
	view := uint64(1 + g.random.Uint32())

	contributingSignerCount := g.random.IntInRange(3, 7)
	signerIndices := g.signerIndices.Fixture(SignerIndices.WithSignerCount(10, contributingSignerCount))

	newestQC := g.quorumCerts.Fixture(QuorumCertificate.WithView(view - 1))

	// newestQCViews should have an entry for each contributing signer, use the same view for each
	newestQCViews := slices.Fill(newestQC.View, contributingSignerCount)

	// Create timeout certificate using constructor to ensure validity
	tc := &flow.TimeoutCertificate{
		View:          view,
		NewestQCViews: newestQCViews,
		NewestQC:      newestQC,
		SignerIndices: signerIndices,
		SigData:       g.signatures.Fixture(),
	}

	for _, opt := range opts {
		opt(g, tc)
	}

	return tc
}

// List generates a list of [flow.TimeoutCertificate].
func (g *TimeoutCertificateGenerator) List(n int, opts ...TimeoutCertificateOption) []*flow.TimeoutCertificate {
	certificates := make([]*flow.TimeoutCertificate, n)
	for i := range n {
		certificates[i] = g.Fixture(opts...)
	}
	return certificates
}
