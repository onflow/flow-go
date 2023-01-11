// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test the canonical ordering of identity and identifier match
func TestCanonicalOrderingMatch(t *testing.T) {
	identities := unittest.IdentityListFixture(100)
	require.Equal(t, identities.Sort(order.Canonical).NodeIDs(), identities.NodeIDs().Sort(order.IdentifierCanonical))
}
