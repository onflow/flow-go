package id

import (
	"math/rand"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestFixedIdentifierProvider(t *testing.T) {
	identifiers := make([]flow.Identifier, 10)
	for i := 0; i < len(identifiers); i++ {
		identifiers[i] = unittest.IdentifierFixture()
	}

	fp := NewFixedIdentifierProvider(identifiers)

	in := identifiers[rand.Intn(10)]
	out := unittest.IdentifierFixture()

	assert.True(t, contains(fp.Identifiers(), in))
	assert.False(t, contains(fp.Identifiers(), out))

}

func TestFixedIdentitiesProvider(t *testing.T) {
	identities := make([]*flow.Identity, 10)
	for i := 0; i < len(identities); i++ {
		identities[i] = unittest.IdentityFixture()
	}

	fp := NewFixedIdentityProvider(identities)

	in := identities[rand.Intn(10)]
	out := unittest.IdentityFixture()

	assert.True(t, idContains(fp.Identities(filter.Any), in))
	assert.False(t, idContains(fp.Identities(filter.Any), out))

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
