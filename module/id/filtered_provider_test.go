package id

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestFilteredIdentitiesProvider(t *testing.T) {
	identities := make([]*flow.Identity, 10)
	for i := 0; i < len(identities); i++ {
		identities[i] = unittest.IdentityFixture()
	}
	identifiers := (flow.IdentityList)(identities).NodeIDs()

	oddIdentifiers := make([]flow.Identifier, 5)
	for j := 0; j < 5; j++ {
		oddIdentifiers[j] = identifiers[2*j+1]
	}

	oddIdentities := make([]*flow.Identity, 5)
	for j := 0; j < 5; j++ {
		oddIdentities[j] = identities[2*j+1]
	}

	ip := NewFixedIdentityProvider(identities)
	fp := NewFilteredIdentifierProvider(filter.In(oddIdentities), ip)

	assert.ElementsMatch(t, fp.Identifiers(),
		(flow.IdentityList)(oddIdentities).NodeIDs())

	in := 0
	out := 0
	for _, id := range identifiers {
		if contains(fp.Identifiers(), id) {
			in++
		} else {
			out++
		}
	}
	require.Equal(t, 5, in)
	require.Equal(t, 5, out)

}
