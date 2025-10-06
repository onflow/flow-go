package fixtures

import (
	"github.com/onflow/crypto"
)

// Signature is the default options factory for [crypto.Signature] generation.
var Signature signatureFactory

type signatureFactory struct{}

type SignatureOption func(*SignatureGenerator, *signatureConfig)

// SignatureGenerator generates signatures with consistent randomness.
type SignatureGenerator struct {
	signatureFactory //nolint:unused

	random *RandomGenerator
}

func NewSignatureGenerator(
	random *RandomGenerator,
) *SignatureGenerator {
	return &SignatureGenerator{
		random: random,
	}
}

// signatureConfig holds the configuration for signature generation.
type signatureConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// Fixture generates a random [crypto.Signature].
func (g *SignatureGenerator) Fixture(opts ...SignatureOption) crypto.Signature {
	config := &signatureConfig{}

	for _, opt := range opts {
		opt(g, config)
	}

	return g.random.RandomBytes(crypto.SignatureLenBLSBLS12381)
}

// List generates a list of random [crypto.Signature].
func (g *SignatureGenerator) List(n int, opts ...SignatureOption) []crypto.Signature {
	sigs := make([]crypto.Signature, n)
	for i := range n {
		sigs[i] = g.Fixture(opts...)
	}
	return sigs
}
