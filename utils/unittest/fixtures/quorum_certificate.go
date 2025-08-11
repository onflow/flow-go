package fixtures

import (
	"testing"

	"github.com/stretchr/testify/require"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// QuorumCertificateGenerator generates quorum certificates with consistent randomness.
type QuorumCertificateGenerator struct {
	randomGen        *RandomGenerator
	identifierGen    *IdentifierGenerator
	signerIndicesGen *SignerIndicesGenerator
	signatureGen     *SignatureGenerator
}

// quorumCertificateConfig holds the configuration for quorum certificate generation.
type quorumCertificateConfig struct {
	view          uint64
	blockID       flow.Identifier
	signerIndices []byte
	source        []byte
}

// WithView returns an option to set the view of the quorum certificate.
func (g *QuorumCertificateGenerator) WithView(view uint64) func(*quorumCertificateConfig) {
	return func(config *quorumCertificateConfig) {
		config.view = view
	}
}

// WithRootBlockID returns an option to set the root block ID of the quorum certificate.
func (g *QuorumCertificateGenerator) WithRootBlockID(blockID flow.Identifier) func(*quorumCertificateConfig) {
	return func(config *quorumCertificateConfig) {
		config.blockID = blockID
		config.view = 0
	}
}

// WithBlockID returns an option to set the block ID of the quorum certificate.
func (g *QuorumCertificateGenerator) WithBlockID(blockID flow.Identifier) func(*quorumCertificateConfig) {
	return func(config *quorumCertificateConfig) {
		config.blockID = blockID
	}
}

func (g *QuorumCertificateGenerator) CertifiesBlock(header *flow.Header) func(*quorumCertificateConfig) {
	return func(config *quorumCertificateConfig) {
		config.view = header.View
		config.blockID = header.ID()
	}
}

// WithSignerIndices returns an option to set the signer indices of the quorum certificate.
func (g *QuorumCertificateGenerator) WithSignerIndices(signerIndices []byte) func(*quorumCertificateConfig) {
	return func(config *quorumCertificateConfig) {
		config.signerIndices = signerIndices
	}
}

// WithRandomnessSource returns an option to set the source of randomness for the quorum certificate.
func (g *QuorumCertificateGenerator) WithRandomnessSource(source []byte) func(*quorumCertificateConfig) {
	return func(config *quorumCertificateConfig) {
		config.source = source
	}
}

// Fixture generates a quorum certificate with optional configuration.
func (g *QuorumCertificateGenerator) Fixture(t testing.TB, opts ...func(*quorumCertificateConfig)) *flow.QuorumCertificate {
	config := &quorumCertificateConfig{
		view:          uint64(g.randomGen.Uint32()),
		blockID:       g.identifierGen.Fixture(t),
		signerIndices: g.signerIndicesGen.Fixture(t, 3),
		source:        nil,
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	return &flow.QuorumCertificate{
		View:          config.view,
		BlockID:       config.blockID,
		SignerIndices: config.signerIndices,
		SigData:       g.QCSigDataWithSoR(t, config.source),
	}
}

// List generates a list of quorum certificates.
func (g *QuorumCertificateGenerator) List(t testing.TB, n int, opts ...func(*quorumCertificateConfig)) []*flow.QuorumCertificate {
	list := make([]*flow.QuorumCertificate, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}

// QCSigDataWithSoR generates a quorum certificate signature data with a source of randomness.
// If source is empty, a random source of randomness is generated.
func (g *QuorumCertificateGenerator) QCSigDataWithSoR(t testing.TB, source []byte) []byte {
	packer := hotstuff.SigDataPacker{}
	sigData := g.qcRawSignatureData(t, source)
	encoded, err := packer.Encode(&sigData)
	require.NoError(t, err)
	return encoded
}

// Helper methods for generating signature data

func (g *QuorumCertificateGenerator) qcRawSignatureData(t testing.TB, source []byte) hotstuff.SignatureData {
	sigType := g.randomGen.RandomBytes(t, 5)
	for i := range sigType {
		sigType[i] = sigType[i] % 2
	}
	sigData := hotstuff.SignatureData{
		SigType:                      sigType,
		AggregatedStakingSig:         g.signatureGen.Fixture(t),
		AggregatedRandomBeaconSig:    g.signatureGen.Fixture(t),
		ReconstructedRandomBeaconSig: source,
	}
	if len(sigData.ReconstructedRandomBeaconSig) == 0 {
		sigData.ReconstructedRandomBeaconSig = g.signatureGen.Fixture(t)
	}
	return sigData
}
