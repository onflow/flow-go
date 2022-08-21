package id

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFixedIdentifierProvider ensure the fixed identity provider contains the expected identifiers
func TestFixedIdentifierProvider(t *testing.T) {
	identifiers := make([]flow.Identifier, 10)
	for i := 0; i < len(identifiers); i++ {
		identifiers[i] = unittest.IdentifierFixture()
	}

	fp := NewFixedIdentifierProvider(identifiers)

	in := identifiers[rand.Intn(10)]
	out := unittest.IdentifierFixture()

	require.True(t, contains(fp.Identifiers(), in))
	require.False(t, contains(fp.Identifiers(), out))

}

// TestFixedIdentitiesProvider ensure the fixed identity provider contains the expected identities
func TestFixedIdentitiesProvider(t *testing.T) {
	identities := make([]*flow.Identity, 10)
	for i := 0; i < len(identities); i++ {
		identities[i] = unittest.IdentityFixture()
	}

	fp := NewFixedIdentityProvider(identities)

	in := identities[rand.Intn(10)]
	out := unittest.IdentityFixture()

	require.True(t, idContains(fp.Identities(filter.Any), in))
	require.False(t, idContains(fp.Identities(filter.Any), out))

}

func contains(a []flow.Identifier, b flow.Identifier) bool {
	for _, i := range a {
		if b == i {
			return true
		}
	}
	return false
}

func idContains(a []*flow.Identity, b *flow.Identity) bool {
	for _, i := range a {
		if b == i {
			return true
		}
	}
	return false
}
