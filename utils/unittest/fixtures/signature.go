package fixtures

import (
	"testing"

	"github.com/onflow/crypto"
)

// SignatureGenerator generates signatures with consistent randomness.
type SignatureGenerator struct {
	randomGen *RandomGenerator
}

// signatureConfig holds the configuration for signature generation.
type signatureConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// Fixture generates a random signature.
func (g *SignatureGenerator) Fixture(t testing.TB, opts ...func(*signatureConfig)) crypto.Signature {
	config := &signatureConfig{}

	for _, opt := range opts {
		opt(config)
	}

	return g.randomGen.RandomBytes(t, crypto.SignatureLenBLSBLS12381)
}

// List generates a list of random signatures.
func (g *SignatureGenerator) List(t testing.TB, n int, opts ...func(*signatureConfig)) []crypto.Signature {
	sigs := make([]crypto.Signature, n)
	for i := range n {
		sigs[i] = g.Fixture(t, opts...)
	}
	return sigs
}
