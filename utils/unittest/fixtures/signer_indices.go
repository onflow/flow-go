package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// SignerIndicesGenerator generates signer indices with consistent randomness.
type SignerIndicesGenerator struct {
}

// Fixture generates signer indices with the specified count.
// Uses default 10-bit vector size.
func (g *SignerIndicesGenerator) Fixture(t testing.TB, n int) []byte {
	indices := bitutils.MakeBitVector(10) // default bit size
	for range n {
		bitutils.SetBit(indices, 1)
	}
	return indices
}

// WithIndices generates signer indices at specific positions.
func (g *SignerIndicesGenerator) ByIndices(t testing.TB, indices []int) []byte {
	signers := bitutils.MakeBitVector(10) // default bit size
	for _, i := range indices {
		bitutils.SetBit(signers, i)
	}
	return signers
}

// List generates a list of signer indices.
func (g *SignerIndicesGenerator) List(t testing.TB, n int, count int) [][]byte {
	list := make([][]byte, n)
	for i := range n {
		list[i] = g.Fixture(t, count)
	}
	return list
}
