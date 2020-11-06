package topology_test

import (
	"fmt"
	"testing"

	"github.com/bsipos/thist"
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
}

// TestStatefulTopologyTestSuite runs all tests in this test suite
func TestStatefulTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(StatefulTopologyTestSuite))
}

func (suite *StatefulTopologyTestSuite) TestSingleSystemLowScale() {
	suite.subFanoutScenario(40, 100, 100, 100, 100, 100, 10)
}

// generateSystem is a test helper that given number of nodes per role as well as desire number of clusters
// generates the protocol state, identity list and subscription managers for the nodes.
// - acc: number of access nodes
// - col: number of collection nodes
// - exe: number of execution nodes
// - ver: number of verification nodes
// - cluster: number of clusters of collection nodes
func (suite *StatefulTopologyTestSuite) generateSystem(acc, col, con, exe, ver, cluster int) (protocol.State,
	flow.IdentityList,
	[]channel.SubscriptionManager) {

	collector, _ := test.GenerateIDs(suite.T(), col, test.RunNetwork, unittest.WithRole(flow.RoleCollection))
	access, _ := test.GenerateIDs(suite.T(), acc, test.RunNetwork, unittest.WithRole(flow.RoleAccess))
	consensus, _ := test.GenerateIDs(suite.T(), con, test.RunNetwork, unittest.WithRole(flow.RoleConsensus))
	verification, _ := test.GenerateIDs(suite.T(), ver, test.RunNetwork, unittest.WithRole(flow.RoleVerification))
	execution, _ := test.GenerateIDs(suite.T(), exe, test.RunNetwork, unittest.WithRole(flow.RoleExecution))

	ids := flow.IdentityList{}
	ids = ids.Union(collector)
	ids = ids.Union(access)
	ids = ids.Union(consensus)
	ids = ids.Union(verification)
	ids = ids.Union(execution)

	// mocks state for collector nodes topology
	state, _ := topology.CreateMockStateForCollectionNodes(suite.T(),
		ids.Filter(filter.HasRole(flow.RoleCollection)), uint(cluster))

	subMngrs := test.MockSubscriptionManager(suite.T(), ids)

	return state, ids, subMngrs
}

// subFanoutScenario is a test helper evaluates that sub-fanout of each role in topology is
// bound by the fanout function of topology on the role's entire size.
// For example if fanout function is `n+1/2` and we have x consensus nodes,
// then this tests evaluates that no node should have more than or equal to `x+1/2`
// consensus fanout.
func (suite *StatefulTopologyTestSuite) subFanoutScenario(system, acc, col, con, exe, ver, cluster int) {
	// fanouts := make([]float64, 0, system)
	h := thist.NewHist(nil, "Fanout histogram", "auto", -1, false)

	for j := 0; j < system; j++ {
		state, ids, subMngrs := suite.generateSystem(acc, col, con, exe, ver, cluster)
		fanout := 0 // keeps summation of nodes' fanout for statistical reason

		// creates topology and topology manager
		for i, id := range ids {
			top, err := topology.NewTopicBasedTopology(id.NodeID, state)
			require.NoError(suite.T(), err)

			// creates topology manager
			topMngr := topology.NewStatefulTopologyManager(top, subMngrs[i], topology.LinearFanoutFunc)

			// generates topology of node
			myFanout, err := topMngr.MakeTopology(ids)
			require.NoError(suite.T(), err)

			for _, role := range flow.Roles() {
				// total number of nodes in flow with specified role
				roleTotal := uint(len(ids.Filter(filter.HasRole(role))))
				// number of nodes in fanout with specified role
				roleFanout := topMngr.Fanout(uint(len(myFanout.Filter(filter.HasRole(role)))))
				require.Less(suite.T(), roleFanout, roleTotal)
			}

			fanout += len(myFanout)
		}

		// keeps track of average fanout
		h.Update(float64(fanout) / float64(len(ids)))
	}

	fmt.Println(h.Draw())
}
