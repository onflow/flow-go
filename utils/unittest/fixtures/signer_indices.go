package fixtures

import (
	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// SignerIndices is the default options factory for SignerIndices generation.
var SignerIndices signerIndicesFactory

type signerIndicesFactory struct{}

type SignerIndicesOption func(*SignerIndicesGenerator, *signerIndicesConfig)

// signerIndicesConfig holds the configuration for signer indices generation.
type signerIndicesConfig struct {
	authorizedSigners   int
	contributingSigners int
	indices             []int
}

// WithSignerCount is an option that sets the total number of indices and the number of signers.
func (f signerIndicesFactory) WithSignerCount(authorizedSigners, contributingSigners int) SignerIndicesOption {
	return func(g *SignerIndicesGenerator, config *signerIndicesConfig) {
		config.authorizedSigners = authorizedSigners
		config.contributingSigners = contributingSigners
	}
}

// WithIndices is an option that sets the total number of indices and specific indices for signers.
// Note: passing an empty slice is valid and will be treated as all indices are not set.
func (f signerIndicesFactory) WithIndices(authorizedSigners int, indices []int) SignerIndicesOption {
	return func(g *SignerIndicesGenerator, config *signerIndicesConfig) {
		config.authorizedSigners = authorizedSigners
		config.indices = indices
	}
}

// SignerIndicesGenerator generates signer indices with consistent randomness.
type SignerIndicesGenerator struct {
	signerIndicesFactory

	random *RandomGenerator
}

func NewSignerIndicesGenerator(random *RandomGenerator) *SignerIndicesGenerator {
	return &SignerIndicesGenerator{
		random: random,
	}
}

// Fixture generates signer indices with random data based on the provided options.
// Uses default 10-bit vector size and count of 3 signers.
func (g *SignerIndicesGenerator) Fixture(opts ...SignerIndicesOption) []byte {
	config := &signerIndicesConfig{
		authorizedSigners:   10,
		contributingSigners: 3,
	}

	for _, opt := range opts {
		opt(g, config)
	}

	Assert(config.authorizedSigners > 0, "authorizedSigners must be greater than 0")
	Assert(config.contributingSigners >= 0, "contributingSigners must be greater than or equal to 0")
	Assert(config.contributingSigners <= config.authorizedSigners, "contributingSigners must be less than or equal to authorizedSigners")

	indices := bitutils.MakeBitVector(config.authorizedSigners)

	if config.indices != nil {
		for _, i := range config.indices {
			Assert(i >= 0 && i < config.authorizedSigners, "index must be within the total number of indices")
			bitutils.SetBit(indices, i)
		}
		return indices
	}

	// special case to avoid looping when all indices are set
	if config.contributingSigners == config.authorizedSigners {
		for i := range config.authorizedSigners {
			bitutils.SetBit(indices, i)
		}
		return indices
	}

	// choose `contributingSigners` random indices from the total
	count := 0
	for count < config.contributingSigners {
		index := g.random.Intn(config.authorizedSigners)

		// only count unset bits to ensure that we set the correct number of unique indices
		if bitutils.ReadBit(indices, index) == 0 {
			bitutils.SetBit(indices, index)
			count++
		}
	}

	return indices
}

// List generates a list of signer indices.
// Uses default 10-bit vector size and count of 3 signers.
func (g *SignerIndicesGenerator) List(n int, opts ...SignerIndicesOption) [][]byte {
	list := make([][]byte, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
