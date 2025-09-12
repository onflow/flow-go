package fixtures

import (
	"github.com/onflow/crypto"
)

// SignatureGenerator generates signatures with consistent randomness.
type SignatureGenerator struct {
	randomGen *RandomGenerator
}

func NewSignatureGenerator(
	randomGen *RandomGenerator,
) *SignatureGenerator {
	return &SignatureGenerator{
		randomGen: randomGen,
	}
}

// signatureConfig holds the configuration for signature generation.
type signatureConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// Fixture generates a random [crypto.Signature].
func (g *SignatureGenerator) Fixture(opts ...func(*signatureConfig)) crypto.Signature {
	config := &signatureConfig{}

	for _, opt := range opts {
		opt(config)
	}

	return g.randomGen.RandomBytes(crypto.SignatureLenBLSBLS12381)
}

// List generates a list of random [crypto.Signature].
func (g *SignatureGenerator) List(n int, opts ...func(*signatureConfig)) []crypto.Signature {
	sigs := make([]crypto.Signature, n)
	for i := range n {
		sigs[i] = g.Fixture(opts...)
	}
	return sigs
}
