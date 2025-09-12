package fixtures

import (
	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// SignerIndicesGenerator generates signer indices with consistent randomness.
type SignerIndicesGenerator struct {
	randomGen *RandomGenerator
}

func NewSignerIndicesGenerator(randomGen *RandomGenerator) *SignerIndicesGenerator {
	return &SignerIndicesGenerator{
		randomGen: randomGen,
	}
}

// signerIndicesConfig holds the configuration for signer indices generation.
type signerIndicesConfig struct {
	total   int
	signers int
	indices []int
}

// WithSignerCount is an option that sets the total number of indices and the number of signers.
func (g *SignerIndicesGenerator) WithSignerCount(total, signers int) func(*signerIndicesConfig) {
	return func(config *signerIndicesConfig) {
		config.total = total
		config.signers = signers
	}
}

// WithIndices is an option that sets the total number of indices and specific indices for signers.
// Note: passing an empty slice is valid and will be treated as all indices are not set.
func (g *SignerIndicesGenerator) WithIndices(total int, indices []int) func(*signerIndicesConfig) {
	return func(config *signerIndicesConfig) {
		config.total = total
		config.indices = indices
	}
}

// Fixture generates signer indices with random data based on the provided options.
// Uses default 10-bit vector size and count of 3 signers.
func (g *SignerIndicesGenerator) Fixture(opts ...func(*signerIndicesConfig)) []byte {
	config := &signerIndicesConfig{
		total:   10,
		signers: 3,
	}

	for _, opt := range opts {
		opt(config)
	}

	Assert(config.total > 0, "total must be greater than 0")
	Assert(config.signers <= config.total, "signers must be less than or equal to total")

	indices := bitutils.MakeBitVector(config.total)

	if config.indices != nil {
		for _, i := range config.indices {
			Assert(i >= 0 && i < config.total, "index must be within the total number of indices")
			bitutils.SetBit(indices, i)
		}
		return indices
	}

	// special case to avoid looping when all indices are set
	if config.signers == config.total {
		for i := range config.total {
			bitutils.SetBit(indices, i)
		}
		return indices
	}

	// choose `signers` random indices from the total
	count := 0
	for count < config.signers {
		index := g.randomGen.Intn(config.total)
		// make sure we get the correct number of unique indices
		if bitutils.ReadBit(indices, index) == 0 {
			bitutils.SetBit(indices, index)
			count++
		}
	}

	return indices
}

// List generates a list of signer indices.
// Uses default 10-bit vector size and count of 3 signers.
func (g *SignerIndicesGenerator) List(n int, opts ...func(*signerIndicesConfig)) [][]byte {
	list := make([][]byte, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
