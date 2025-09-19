package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// ProposalKey is the default options factory for [flow.ProposalKey] generation.
var ProposalKey proposalKeyFactory

type proposalKeyFactory struct{}

type ProposalKeyOption func(*ProposalKeyGenerator, *flow.ProposalKey)

// WithAddress is an option that sets the address for the proposal key.
func (f proposalKeyFactory) WithAddress(address flow.Address) ProposalKeyOption {
	return func(g *ProposalKeyGenerator, key *flow.ProposalKey) {
		key.Address = address
	}
}

// WithKeyIndex is an option that sets the key index for the proposal key.
func (f proposalKeyFactory) WithKeyIndex(keyIndex uint32) ProposalKeyOption {
	return func(g *ProposalKeyGenerator, key *flow.ProposalKey) {
		key.KeyIndex = keyIndex
	}
}

// WithSequenceNumber is an option that sets the sequence number for the proposal key.
func (f proposalKeyFactory) WithSequenceNumber(sequenceNumber uint64) ProposalKeyOption {
	return func(g *ProposalKeyGenerator, key *flow.ProposalKey) {
		key.SequenceNumber = sequenceNumber
	}
}

// ProposalKeyGenerator generates proposal keys with consistent randomness.
type ProposalKeyGenerator struct {
	proposalKeyFactory

	addresses *AddressGenerator
}

func NewProposalKeyGenerator(
	addresses *AddressGenerator,
) *ProposalKeyGenerator {
	return &ProposalKeyGenerator{
		addresses: addresses,
	}
}

// Fixture generates a [flow.ProposalKey] with random data based on the provided options.
func (g *ProposalKeyGenerator) Fixture(opts ...ProposalKeyOption) flow.ProposalKey {
	key := flow.ProposalKey{
		Address:        g.addresses.Fixture(),
		KeyIndex:       1,
		SequenceNumber: 0,
	}

	for _, opt := range opts {
		opt(g, &key)
	}

	return key
}

// List generates a list of [flow.ProposalKey].
func (g *ProposalKeyGenerator) List(n int, opts ...ProposalKeyOption) []flow.ProposalKey {
	list := make([]flow.ProposalKey, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
