package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// IdentifierGenerator generates identifiers with consistent randomness.
type IdentifierGenerator struct {
	randomGen *RandomGenerator
}

func NewIdentifierGenerator(
	randomGen *RandomGenerator,
) *IdentifierGenerator {
	return &IdentifierGenerator{
		randomGen: randomGen,
	}
}

// identifierConfig holds the configuration for identifier generation.
type identifierConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// Fixture generates a random [flow.Identifier].
func (g *IdentifierGenerator) Fixture(opts ...func(*identifierConfig)) flow.Identifier {
	config := &identifierConfig{}

	for _, opt := range opts {
		opt(config)
	}

	id, err := flow.ByteSliceToId(g.randomGen.RandomBytes(flow.IdentifierLen))
	NoError(err)
	return id
}

// List generates a list of random [flow.Identifier].
func (g *IdentifierGenerator) List(n int, opts ...func(*identifierConfig)) flow.IdentifierList {
	list := make([]flow.Identifier, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
