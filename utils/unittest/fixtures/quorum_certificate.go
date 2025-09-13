package fixtures

import (
	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// QuorumCertificate is the default options factory for [flow.QuorumCertificate] generation.
var QuorumCertificate quorumCertificateFactory

type quorumCertificateFactory struct{}

type QuorumCertificateOption func(*QuorumCertificateGenerator, *flow.QuorumCertificate)

// WithView is an option that sets the view of the quorum certificate.
func (f quorumCertificateFactory) WithView(view uint64) QuorumCertificateOption {
	return func(g *QuorumCertificateGenerator, qc *flow.QuorumCertificate) {
		qc.View = view
	}
}

// WithRootBlockID is an option that sets the root block ID of the quorum certificate.
func (f quorumCertificateFactory) WithRootBlockID(blockID flow.Identifier) QuorumCertificateOption {
	return func(g *QuorumCertificateGenerator, qc *flow.QuorumCertificate) {
		qc.BlockID = blockID
		qc.View = 0
	}
}

// WithBlockID is an option that sets the block ID of the quorum certificate.
func (f quorumCertificateFactory) WithBlockID(blockID flow.Identifier) QuorumCertificateOption {
	return func(g *QuorumCertificateGenerator, qc *flow.QuorumCertificate) {
		qc.BlockID = blockID
	}
}

// CertifiesBlock is an option that sets the block ID and view of the quorum certificate to match
// the provided header.
func (f quorumCertificateFactory) CertifiesBlock(header *flow.Header) QuorumCertificateOption {
	return func(g *QuorumCertificateGenerator, qc *flow.QuorumCertificate) {
		qc.View = header.View
		qc.BlockID = header.ID()
	}
}

// WithSignerIndices is an option that sets the signer indices of the quorum certificate.
func (f quorumCertificateFactory) WithSignerIndices(signerIndices []byte) QuorumCertificateOption {
	return func(g *QuorumCertificateGenerator, qc *flow.QuorumCertificate) {
		qc.SignerIndices = signerIndices
	}
}

// WithRandomnessSource is an option that sets the source of randomness for the quorum certificate.
func (f quorumCertificateFactory) WithRandomnessSource(source []byte) QuorumCertificateOption {
	return func(g *QuorumCertificateGenerator, qc *flow.QuorumCertificate) {
		qc.SigData = g.QCSigDataWithSoR(source)
	}
}

// QuorumCertificateGenerator generates quorum certificates with consistent randomness.
type QuorumCertificateGenerator struct {
	quorumCertificateFactory

	randomGen        *RandomGenerator
	identifierGen    *IdentifierGenerator
	signerIndicesGen *SignerIndicesGenerator
	signatureGen     *SignatureGenerator
}

func NewQuorumCertificateGenerator(
	randomGen *RandomGenerator,
	identifierGen *IdentifierGenerator,
	signerIndicesGen *SignerIndicesGenerator,
	signatureGen *SignatureGenerator,
) *QuorumCertificateGenerator {
	return &QuorumCertificateGenerator{
		randomGen:        randomGen,
		identifierGen:    identifierGen,
		signerIndicesGen: signerIndicesGen,
		signatureGen:     signatureGen,
	}
}

// Fixture generates a [flow.QuorumCertificate] with random data based on the provided options.
func (g *QuorumCertificateGenerator) Fixture(opts ...QuorumCertificateOption) *flow.QuorumCertificate {
	qc := &flow.QuorumCertificate{
		View:          uint64(g.randomGen.Uint32()),
		BlockID:       g.identifierGen.Fixture(),
		SignerIndices: g.signerIndicesGen.Fixture(SignerIndices.WithSignerCount(10, 3)),
		SigData:       g.QCSigDataWithSoR(nil),
	}

	for _, opt := range opts {
		opt(g, qc)
	}

	return qc
}

// List generates a list of [flow.QuorumCertificate].
func (g *QuorumCertificateGenerator) List(n int, opts ...QuorumCertificateOption) []*flow.QuorumCertificate {
	list := make([]*flow.QuorumCertificate, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}

// QCSigDataWithSoR generates a [flow.QuorumCertificate] signature data with a source of randomness.
// If source is empty, a random source of randomness is generated.
func (g *QuorumCertificateGenerator) QCSigDataWithSoR(source []byte) []byte {
	packer := hotstuff.SigDataPacker{}
	sigData := g.qcRawSignatureData(source)
	encoded, err := packer.Encode(&sigData)
	NoError(err)
	return encoded
}

// qcRawSignatureData generates a raw signature data for a [flow.QuorumCertificate].
func (g *QuorumCertificateGenerator) qcRawSignatureData(source []byte) hotstuff.SignatureData {
	sigType := g.randomGen.RandomBytes(5)
	for i := range sigType {
		sigType[i] = sigType[i] % 2
	}
	sigData := hotstuff.SignatureData{
		SigType:                      sigType,
		AggregatedStakingSig:         g.signatureGen.Fixture(),
		AggregatedRandomBeaconSig:    g.signatureGen.Fixture(),
		ReconstructedRandomBeaconSig: source,
	}
	if len(sigData.ReconstructedRandomBeaconSig) == 0 {
		sigData.ReconstructedRandomBeaconSig = g.signatureGen.Fixture()
	}
	return sigData
}
