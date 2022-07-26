package topology_test

import (
	"testing"

	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

// TestEmptyTopology checks that FullyConnectedTopology always returns the input list as the fanout.
func TestFullTopology(t *testing.T) {
	ids := unittest.IdentityListFixture(10)
	top := topology.NewFullyConnectedTopology()
	require.Equal(t, ids, top.Fanout(ids))
}
