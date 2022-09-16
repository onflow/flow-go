package topology_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFullTopology checks that FullyConnectedTopology always returns the input list as the fanout.
func TestFullTopology(t *testing.T) {
	ids := unittest.IdentityListFixture(10)
	top := topology.NewFullyConnectedTopology()
	require.Equal(t, ids, top.Fanout(ids))
}
