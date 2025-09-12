package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// ProposalKeyGenerator generates proposal keys with consistent randomness.
type ProposalKeyGenerator struct {
	addressGen *AddressGenerator
}

func NewProposalKeyGenerator(
	addressGen *AddressGenerator,
) *ProposalKeyGenerator {
	return &ProposalKeyGenerator{
		addressGen: addressGen,
	}
}

// proposalKeyConfig holds the configuration for proposal key generation.
type proposalKeyConfig struct {
	address        flow.Address
	keyIndex       uint32
	sequenceNumber uint64
}

// WithAddress is an option that sets the address for the proposal key.
func (g *ProposalKeyGenerator) WithAddress(address flow.Address) func(*flow.ProposalKey) {
	return func(key *flow.ProposalKey) {
		key.Address = address
	}
}

// WithKeyIndex is an option that sets the key index for the proposal key.
func (g *ProposalKeyGenerator) WithKeyIndex(keyIndex uint32) func(*flow.ProposalKey) {
	return func(key *flow.ProposalKey) {
		key.KeyIndex = keyIndex
	}
}

// WithSequenceNumber is an option that sets the sequence number for the proposal key.
func (g *ProposalKeyGenerator) WithSequenceNumber(sequenceNumber uint64) func(*flow.ProposalKey) {
	return func(key *flow.ProposalKey) {
		key.SequenceNumber = sequenceNumber
	}
}

// Fixture generates a [flow.ProposalKey] with random data based on the provided options.
func (g *ProposalKeyGenerator) Fixture(opts ...func(*flow.ProposalKey)) flow.ProposalKey {
	key := flow.ProposalKey{
		Address:        g.addressGen.Fixture(),
		KeyIndex:       1,
		SequenceNumber: 0,
	}

	for _, opt := range opts {
		opt(&key)
	}

	return key
}

// List generates a list of [flow.ProposalKey].
func (g *ProposalKeyGenerator) List(n int, opts ...func(*flow.ProposalKey)) []flow.ProposalKey {
	list := make([]flow.ProposalKey, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
