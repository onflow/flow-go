package fixtures

import (
	"github.com/onflow/flow-go/model/flow"
)

// Identifier is the default options factory for [flow.Identifier] generation.
var Identifier identifierFactory

type identifierFactory struct{}

type IdentifierOption func(*IdentifierGenerator, *identifierConfig)

// identifierConfig holds the configuration for identifier generation.
type identifierConfig struct {
	// Currently no special options needed, but maintaining pattern consistency
}

// IdentifierGenerator generates identifiers with consistent randomness.
type IdentifierGenerator struct {
	identifierFactory //nolint:unused

	random *RandomGenerator
}

func NewIdentifierGenerator(
	random *RandomGenerator,
) *IdentifierGenerator {
	return &IdentifierGenerator{
		random: random,
	}
}

// Fixture generates a random [flow.Identifier].
func (g *IdentifierGenerator) Fixture(opts ...IdentifierOption) flow.Identifier {
	config := &identifierConfig{}

	for _, opt := range opts {
		opt(g, config)
	}

	id, err := flow.ByteSliceToId(g.random.RandomBytes(flow.IdentifierLen))
	NoError(err)
	return id
}

// List generates a list of random [flow.Identifier].
func (g *IdentifierGenerator) List(n int, opts ...IdentifierOption) flow.IdentifierList {
	list := make([]flow.Identifier, n)
	for i := range n {
		list[i] = g.Fixture(opts...)
	}
	return list
}
