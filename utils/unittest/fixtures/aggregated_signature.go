package fixtures

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// AggregatedSignature is the default options factory for [flow.AggregatedSignature] generation.
var AggregatedSignature aggregatedSignatureFactory

type aggregatedSignatureFactory struct{}

type AggregatedSignatureOption func(*AggregatedSignatureGenerator, *flow.AggregatedSignature)

// WithVerifierSignatures is an option that sets the `VerifierSignatures` of the aggregated signature.
func (f aggregatedSignatureFactory) WithVerifierSignatures(sigs ...crypto.Signature) AggregatedSignatureOption {
	return func(g *AggregatedSignatureGenerator, aggSig *flow.AggregatedSignature) {
		aggSig.VerifierSignatures = sigs
	}
}

// WithSignerIDs is an option that sets the `SignerIDs` of the aggregated signature.
func (f aggregatedSignatureFactory) WithSignerIDs(signerIDs flow.IdentifierList) AggregatedSignatureOption {
	return func(g *AggregatedSignatureGenerator, aggSig *flow.AggregatedSignature) {
		aggSig.SignerIDs = signerIDs
	}
}

// AggregatedSignatureGenerator generates aggregated signatures with consistent randomness.
type AggregatedSignatureGenerator struct {
	aggregatedSignatureFactory

	random      *RandomGenerator
	identifiers *IdentifierGenerator
	signatures  *SignatureGenerator
}

// NewAggregatedSignatureGenerator creates a new AggregatedSignatureGenerator.
func NewAggregatedSignatureGenerator(
	random *RandomGenerator,
	identifiers *IdentifierGenerator,
	signatures *SignatureGenerator,
) *AggregatedSignatureGenerator {
	return &AggregatedSignatureGenerator{
		random:      random,
		identifiers: identifiers,
		signatures:  signatures,
	}
}

// Fixture generates a [flow.AggregatedSignature] with random data based on the provided options.
func (g *AggregatedSignatureGenerator) Fixture(opts ...AggregatedSignatureOption) flow.AggregatedSignature {
	numSigners := g.random.IntInRange(2, 5)
	aggSig := flow.AggregatedSignature{
		VerifierSignatures: g.signatures.List(numSigners),
		SignerIDs:          g.identifiers.List(numSigners),
	}

	for _, opt := range opts {
		opt(g, &aggSig)
	}

	Assertf(len(aggSig.VerifierSignatures) == len(aggSig.SignerIDs), "verifier signatures and signer IDs must have the same length")

	return aggSig
}

// List generates a list of [flow.AggregatedSignature].
func (g *AggregatedSignatureGenerator) List(n int, opts ...AggregatedSignatureOption) []flow.AggregatedSignature {
	signatures := make([]flow.AggregatedSignature, n)
	for i := range n {
		signatures[i] = g.Fixture(opts...)
	}
	return signatures
}
