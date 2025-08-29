package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// SignerIndicesGenerator generates signer indices with consistent randomness.
type SignerIndicesGenerator struct {
}

// signerIndicesConfig holds the configuration for signer indices generation.
type signerIndicesConfig struct {
	total   int
	signers int
	indices []int
}

// WithSignerCount returns an option to set the total number of indices and the number of signers.
func (g *SignerIndicesGenerator) WithSignerCount(indicesCount, signers int) func(*signerIndicesConfig) {
	return func(config *signerIndicesConfig) {
		if indicesCount < signers {
			panic("total must be greater than or equal to signers")
		}
		config.total = indicesCount
		config.signers = signers
	}
}

// WithIndices returns an option to set specific indices for signers.
func (g *SignerIndicesGenerator) WithIndices(indices []int) func(*signerIndicesConfig) {
	return func(config *signerIndicesConfig) {
		config.indices = indices
	}
}

// Fixture generates signer indices with optional configuration.
// Uses default 10-bit vector size and count of 3 signers.
func (g *SignerIndicesGenerator) Fixture(t testing.TB, opts ...func(*signerIndicesConfig)) []byte {
	config := &signerIndicesConfig{
		total:   10, // default total
		signers: 3,  // default signers
	}

	for _, opt := range opts {
		opt(config)
	}

	indices := bitutils.MakeBitVector(config.total)

	if config.indices != nil {
		for _, i := range config.indices {
			bitutils.SetBit(indices, i)
		}
	} else {
		// default to first N signers
		for i := range config.signers {
			bitutils.SetBit(indices, i)
		}
	}

	return indices
}

// List generates a list of signer indices.
func (g *SignerIndicesGenerator) List(t testing.TB, n int, opts ...func(*signerIndicesConfig)) [][]byte {
	list := make([][]byte, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}
