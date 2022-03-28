// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order_test

import (
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// Test the canonical ordering of identity and identifier match
func TestCanonicalOrderingMatch(t *testing.T) {
	identities := unittest.IdentityListFixture()
	require.Equal(t, identities.Sort(Canonical).NodeIDs(), identities.NodeIDs().Sort(IdentifierCanonical))
}
