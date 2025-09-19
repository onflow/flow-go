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

	random        *RandomGenerator
	identifiers   *IdentifierGenerator
	signerIndices *SignerIndicesGenerator
	signatures    *SignatureGenerator
}

func NewQuorumCertificateGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	signerIndices *SignerIndicesGenerator,
	signatures *SignatureGenerator,
) *QuorumCertificateGenerator {
	return &QuorumCertificateGenerator{
		random:        random,
		identifiers:   identifiers,
		signerIndices: signerIndices,
		signatures:    signatures,
	}
}

// Fixture generates a [flow.QuorumCertificate] with random data based on the provided options.
func (g *QuorumCertificateGenerator) Fixture(opts ...QuorumCertificateOption) *flow.QuorumCertificate {
	qc := &flow.QuorumCertificate{
		View:          uint64(g.random.Uint32()),
		BlockID:       g.identifiers.Fixture(),
		SignerIndices: g.signerIndices.Fixture(SignerIndices.WithSignerCount(10, 3)),
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
	sigType := g.random.RandomBytes(5)
	for i := range sigType {
		sigType[i] = sigType[i] % 2
	}
	sigData := hotstuff.SignatureData{
		SigType:                      sigType,
		AggregatedStakingSig:         g.signatures.Fixture(),
		AggregatedRandomBeaconSig:    g.signatures.Fixture(),
		ReconstructedRandomBeaconSig: source,
	}
	if len(sigData.ReconstructedRandomBeaconSig) == 0 {
		sigData.ReconstructedRandomBeaconSig = g.signatures.Fixture()
	}
	return sigData
}

// QuorumCertificateWithSignerIDs is the default options factory for
// [flow.QuorumCertificateWithSignerIDs] generation.
var QuorumCertificateWithSignerIDs quorumCertificateWithSignerIDsFactory

type quorumCertificateWithSignerIDsFactory struct{}

type QuorumCertificateWithSignerIDsOption func(*QuorumCertificateWithSignerIDsGenerator, *flow.QuorumCertificateWithSignerIDs)

// WithView is an option that sets the `View` of the quorum certificate with signer IDs.
func (f quorumCertificateWithSignerIDsFactory) WithView(view uint64) QuorumCertificateWithSignerIDsOption {
	return func(g *QuorumCertificateWithSignerIDsGenerator, qc *flow.QuorumCertificateWithSignerIDs) {
		qc.View = view
	}
}

// WithSignerIDs is an option that sets the `SignerIDs` of the quorum certificate with signer IDs.
func (f quorumCertificateWithSignerIDsFactory) WithSignerIDs(signerIDs flow.IdentifierList) QuorumCertificateWithSignerIDsOption {
	return func(g *QuorumCertificateWithSignerIDsGenerator, qc *flow.QuorumCertificateWithSignerIDs) {
		qc.SignerIDs = signerIDs
	}
}

// WithBlockID is an option that sets the `BlockID` of the quorum certificate with signer IDs.
func (f quorumCertificateWithSignerIDsFactory) WithBlockID(blockID flow.Identifier) QuorumCertificateWithSignerIDsOption {
	return func(g *QuorumCertificateWithSignerIDsGenerator, qc *flow.QuorumCertificateWithSignerIDs) {
		qc.BlockID = blockID
	}
}

// WithSigData is an option that sets the `SigData` of the quorum certificate with signer IDs.
func (f quorumCertificateWithSignerIDsFactory) WithSigData(sigData []byte) QuorumCertificateWithSignerIDsOption {
	return func(g *QuorumCertificateWithSignerIDsGenerator, qc *flow.QuorumCertificateWithSignerIDs) {
		qc.SigData = sigData
	}
}

// QuorumCertificateWithSignerIDsGenerator generates [flow.QuorumCertificateWithSignerIDs] with
// consistent randomness.
type QuorumCertificateWithSignerIDsGenerator struct {
	quorumCertificateWithSignerIDsFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
	quorumCerts *QuorumCertificateGenerator
}

func NewQuorumCertificateWithSignerIDsGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	quorumCerts *QuorumCertificateGenerator,
) *QuorumCertificateWithSignerIDsGenerator {
	return &QuorumCertificateWithSignerIDsGenerator{
		random:      random,
		identifiers: identifiers,
		quorumCerts: quorumCerts,
	}
}

// Fixture generates a [flow.QuorumCertificateWithSignerIDs] with random data based on the provided options.
func (g *QuorumCertificateWithSignerIDsGenerator) Fixture(opts ...QuorumCertificateWithSignerIDsOption) *flow.QuorumCertificateWithSignerIDs {
	qc := &flow.QuorumCertificateWithSignerIDs{
		View:      uint64(g.random.Uint32()),
		BlockID:   g.identifiers.Fixture(),
		SignerIDs: g.identifiers.List(10),
		SigData:   g.quorumCerts.QCSigDataWithSoR(nil),
	}

	for _, opt := range opts {
		opt(g, qc)
	}

	return qc
}

// List generates a list of [flow.QuorumCertificateWithSignerIDs].
func (g *QuorumCertificateWithSignerIDsGenerator) List(n int, opts ...QuorumCertificateWithSignerIDsOption) []*flow.QuorumCertificateWithSignerIDs {
	list := make([]*flow.QuorumCertificateWithSignerIDs, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
