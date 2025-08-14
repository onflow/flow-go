package fixtures

import (
	"testing"

	"github.com/onflow/crypto"
)

// SignatureGenerator generates signatures with consistent randomness.
type SignatureGenerator struct {
	randomGen *RandomGenerator
}

// Fixture generates a random signature.
func (g *SignatureGenerator) Fixture(t testing.TB) crypto.Signature {
	return g.randomGen.RandomBytes(t, crypto.SignatureLenBLSBLS12381)
}

// List generates a list of random signatures.
func (g *SignatureGenerator) List(t testing.TB, n int) []crypto.Signature {
	sigs := make([]crypto.Signature, n)
	for i := range n {
		sigs[i] = g.Fixture(t)
	}
	return sigs
}
