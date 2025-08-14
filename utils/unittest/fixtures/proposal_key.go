package fixtures

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
)

// ProposalKeyGenerator generates proposal keys with consistent randomness.
type ProposalKeyGenerator struct {
	addressGen *AddressGenerator
}

// proposalKeyConfig holds the configuration for proposal key generation.
type proposalKeyConfig struct {
	address        flow.Address
	keyIndex       uint32
	sequenceNumber uint64
}

// WithAddress returns an option to set the address for the proposal key.
func (g *ProposalKeyGenerator) WithAddress(address flow.Address) func(*proposalKeyConfig) {
	return func(config *proposalKeyConfig) {
		config.address = address
	}
}

// WithKeyIndex returns an option to set the key index for the proposal key.
func (g *ProposalKeyGenerator) WithKeyIndex(keyIndex uint32) func(*proposalKeyConfig) {
	return func(config *proposalKeyConfig) {
		config.keyIndex = keyIndex
	}
}

// WithSequenceNumber returns an option to set the sequence number for the proposal key.
func (g *ProposalKeyGenerator) WithSequenceNumber(sequenceNumber uint64) func(*proposalKeyConfig) {
	return func(config *proposalKeyConfig) {
		config.sequenceNumber = sequenceNumber
	}
}

// Fixture generates a proposal key with optional configuration.
func (g *ProposalKeyGenerator) Fixture(t testing.TB, opts ...func(*proposalKeyConfig)) flow.ProposalKey {
	config := &proposalKeyConfig{
		address:        g.addressGen.Fixture(t),
		keyIndex:       1,
		sequenceNumber: 0,
	}

	for _, opt := range opts {
		opt(config)
	}

	return flow.ProposalKey{
		Address:        config.address,
		KeyIndex:       config.keyIndex,
		SequenceNumber: config.sequenceNumber,
	}
}

// List generates a list of proposal keys.
func (g *ProposalKeyGenerator) List(t testing.TB, n int, opts ...func(*proposalKeyConfig)) []flow.ProposalKey {
	list := make([]flow.ProposalKey, n)
	for i := range n {
		list[i] = g.Fixture(t, opts...)
	}
	return list
}
