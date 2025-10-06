package fixtures

import (
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/model/flow"
)

// StateCommitment is the default options factory for [flow.StateCommitment] generation.
var StateCommitment stateCommitmentFactory

type stateCommitmentFactory struct{}

type StateCommitmentOption func(*StateCommitmentGenerator, *flow.StateCommitment)

// WithHash is an option that sets the hash of the state commitment.
func (f stateCommitmentFactory) WithHash(h hash.Hash) StateCommitmentOption {
	return func(g *StateCommitmentGenerator, sc *flow.StateCommitment) {
		*sc = flow.StateCommitment(h)
	}
}

// WithEmptyState is an option that sets the state commitment to empty.
func (f stateCommitmentFactory) WithEmptyState() StateCommitmentOption {
	return func(g *StateCommitmentGenerator, sc *flow.StateCommitment) {
		*sc = flow.EmptyStateCommitment
	}
}

// StateCommitmentGenerator generates state commitments with consistent randomness.
type StateCommitmentGenerator struct {
	stateCommitmentFactory
	random *RandomGenerator
}

// NewStateCommitmentGenerator creates a new StateCommitmentGenerator.
func NewStateCommitmentGenerator(random *RandomGenerator) *StateCommitmentGenerator {
	return &StateCommitmentGenerator{
		random: random,
	}
}

// Fixture generates a [flow.StateCommitment] with random data based on the provided options.
func (g *StateCommitmentGenerator) Fixture(opts ...StateCommitmentOption) flow.StateCommitment {
	var sc flow.StateCommitment

	// Generate random 32-byte hash
	randomBytes := g.random.RandomBytes(32)
	copy(sc[:], randomBytes)

	for _, opt := range opts {
		opt(g, &sc)
	}

	return sc
}

// List generates a list of [flow.StateCommitment].
func (g *StateCommitmentGenerator) List(n int, opts ...StateCommitmentOption) []flow.StateCommitment {
	commitments := make([]flow.StateCommitment, n)
	for i := range n {
		commitments[i] = g.Fixture(opts...)
	}
	return commitments
}
