package topology_test

import (
	"testing"

	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// TestEmptyTopology checks that EmptyTopology always creates an empty list of fanout.
func TestEmptyTopology(t *testing.T) {
	ids := unittest.IdentityListFixture(10)
	top := topology.NewEmptyTopology()
	require.Empty(t, top.Fanout(ids))
}
