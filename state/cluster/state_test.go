package cluster_test

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestIsCanonicalClusterID verifies that cluster ChainIDs generated
// by [CanonicalClusterID] are accepted by [IsCanonicalClusterID], and
// that the standard consensus chainIDs are not accepted.
func TestIsCanonicalClusterID(t *testing.T) {
	for _, chainID := range flow.AllChainIDs() {
		require.False(t, cluster.IsCanonicalClusterID(chainID))
	}
	for range 100 {
		epoch := rand.Uint64()
		clusterID := cluster.CanonicalClusterID(epoch, unittest.IdentifierListFixture(2))
		require.True(t, cluster.IsCanonicalClusterID(clusterID))
		require.True(t, strings.HasPrefix(string(clusterID), cluster.ClusterChainPrefix))
	}
}
