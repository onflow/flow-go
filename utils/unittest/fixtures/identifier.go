package fixtures

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// IdentifierGenerator generates identifiers with consistent randomness.
type IdentifierGenerator struct {
	randomGen *RandomGenerator
}

// Fixture generates a random identifier.
func (g *IdentifierGenerator) Fixture(t testing.TB) flow.Identifier {
	id, err := flow.ByteSliceToId(g.randomGen.RandomBytes(t, flow.IdentifierLen))
	require.NoError(t, err)
	return id
}

// List generates a list of random identifiers.
func (g *IdentifierGenerator) List(t testing.TB, n int) flow.IdentifierList {
	list := make([]flow.Identifier, n)
	for i := range n {
		list[i] = g.Fixture(t)
	}
	return list
}
