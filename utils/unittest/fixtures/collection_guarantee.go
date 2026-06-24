package fixtures

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/cluster"
)

// CollectionGuarantee is the default options factory for [flow.CollectionGuarantee] generation.
var Guarantee collectionGuaranteeFactory

type collectionGuaranteeFactory struct{}

type CollectionGuaranteeOption func(*CollectionGuaranteeGenerator, *flow.CollectionGuarantee)

// WithCollectionID is an option that sets the `CollectionID` of the collection guarantee.
func (f collectionGuaranteeFactory) WithCollectionID(collectionID flow.Identifier) CollectionGuaranteeOption {
	return func(g *CollectionGuaranteeGenerator, guarantee *flow.CollectionGuarantee) {
		guarantee.CollectionID = collectionID
	}
}

// WithReferenceBlockID is an option that sets the `ReferenceBlockID` of the collection guarantee.
func (f collectionGuaranteeFactory) WithReferenceBlockID(blockID flow.Identifier) CollectionGuaranteeOption {
	return func(g *CollectionGuaranteeGenerator, guarantee *flow.CollectionGuarantee) {
		guarantee.ReferenceBlockID = blockID
	}
}

// WithClusterChainID is an option that sets the `ClusterChainID` of the collection guarantee.
func (f collectionGuaranteeFactory) WithClusterChainID(clusterChainID flow.ChainID) CollectionGuaranteeOption {
	return func(g *CollectionGuaranteeGenerator, guarantee *flow.CollectionGuarantee) {
		guarantee.ClusterChainID = clusterChainID
	}
}

// WithSignerIndices is an option that sets the `SignerIndices` of the collection guarantee.
func (f collectionGuaranteeFactory) WithSignerIndices(signerIndices []byte) CollectionGuaranteeOption {
	return func(g *CollectionGuaranteeGenerator, guarantee *flow.CollectionGuarantee) {
		guarantee.SignerIndices = signerIndices
	}
}

// WithSignature is an option that sets the `Signature` of the collection guarantee.
func (f collectionGuaranteeFactory) WithSignature(signature crypto.Signature) CollectionGuaranteeOption {
	return func(g *CollectionGuaranteeGenerator, guarantee *flow.CollectionGuarantee) {
		guarantee.Signature = signature
	}
}

// CollectionGuaranteeGenerator generates collection guarantees with consistent randomness.
type CollectionGuaranteeGenerator struct {
	collectionGuaranteeFactory

	random        *RandomGenerator
	identifiers   *IdentifierGenerator
	signatures    *SignatureGenerator
	signerIndices *SignerIndicesGenerator

	chainID flow.ChainID
}

func NewCollectionGuaranteeGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	signatures *SignatureGenerator,
	signerIndices *SignerIndicesGenerator,
	chainID flow.ChainID,
) *CollectionGuaranteeGenerator {
	return &CollectionGuaranteeGenerator{
		random:        random,
		identifiers:   identifiers,
		signatures:    signatures,
		signerIndices: signerIndices,
		chainID:       chainID,
	}
}

// Fixture generates a [flow.CollectionGuarantee] with random data based on the provided options.
func (g *CollectionGuaranteeGenerator) Fixture(opts ...CollectionGuaranteeOption) *flow.CollectionGuarantee {
	guarantee := &flow.CollectionGuarantee{
		CollectionID:     g.identifiers.Fixture(),
		ReferenceBlockID: g.identifiers.Fixture(),
		ClusterChainID:   cluster.CanonicalClusterID(g.random.Uint64(), g.identifiers.List(1)),
		SignerIndices:    g.signerIndices.Fixture(),
		Signature:        g.signatures.Fixture(),
	}

	for _, opt := range opts {
		opt(g, guarantee)
	}

	return guarantee
}

// List generates a list of [flow.CollectionGuarantee] with random data.
func (g *CollectionGuaranteeGenerator) List(n int, opts ...CollectionGuaranteeOption) []*flow.CollectionGuarantee {
	guarantees := make([]*flow.CollectionGuarantee, 0, n)
	for i := 0; i < n; i++ {
		guarantees = append(guarantees, g.Fixture(opts...))
	}
	return guarantees
}
