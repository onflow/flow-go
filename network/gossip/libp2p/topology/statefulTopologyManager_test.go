package topology_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/channel"
	"github.com/onflow/flow-go/network/gossip/libp2p/test"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

type StatefulTopologyTestSuite struct {
	suite.Suite
	state    protocol.State                // represents a mocked protocol state
	ids      flow.IdentityList             // represents the identity list of all nodes in the system
	clusters flow.ClusterList              // represents list of cluster ids of collection nodes
	me       flow.Identity                 // represents identity of single instance of node that creates topology
	subMngrs []channel.SubscriptionManager // used to mock subscription manager for topology manager

}

// TestStatefulTopologyTestSuite runs all tests in this test suite
func TestStatefulTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(StatefulTopologyTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (suite *StatefulTopologyTestSuite) SetupTest() {
	// creates 1000 flow nodes
	// including 300 collectors in 10 clusters
	nClusters := 10
	nCollectors := 300
	nTotal := 1000

	collectors, _ := test.GenerateIDs(suite.T(), nCollectors, test.RunNetwork, unittest.WithRole(flow.RoleCollection))
	others, _ := test.GenerateIDs(suite.T(), nTotal-nCollectors, test.RunNetwork,
		unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.ids = append(others, collectors...)

	// mocks state for collector nodes topology
	suite.state, suite.clusters = topology.CreateMockStateForCollectionNodes(suite.T(),
		suite.ids.Filter(filter.HasRole(flow.RoleCollection)), uint(nClusters))

	suite.subMngrs = test.MockSubscriptionManager(suite.T(), suite.ids)

	// takes first id as the current nodes id
	suite.me = *suite.ids[0]
}

// TestRoleSize evaluates that sub-fanout of each role in topology is
// bound by the fanout function of topology on the role's entire size.
// For example if fanout function is `n+1/2` and we have x consensus nodes,
// then this tests evaluates that no node should have more than or equal to `x+1/2`
// consensus faout.
func (suite *StatefulTopologyTestSuite) TestRoleSize() {
	// creates topology and topology manager
	for i, id := range suite.ids {
		top, err := topology.NewTopicBasedTopology(id.NodeID, suite.state)
		require.NoError(suite.T(), err)

		// creates topology manager
		topMngr := topology.NewStatefulTopologyManager(top, suite.subMngrs[i], topology.LinearFanoutFunc)

		// generates topology of node
		myFanout, err := topMngr.MakeTopology(suite.ids)
		require.NoError(suite.T(), err)

		for _, role := range flow.Roles() {
			// total number of nodes in flow with specified role
			roleTotal := uint(len(suite.ids.Filter(filter.HasRole(role))))
			// number of nodes in fanout with specified role
			roleFanout := topMngr.Fanout(uint(len(myFanout.Filter(filter.HasRole(role)))))

			require.LessOrEqual(suite.T(), roleFanout, roleTotal)
		}
	}
}
