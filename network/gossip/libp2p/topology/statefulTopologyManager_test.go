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
	histFlag bool // used to control printing histogram
}

// TestStatefulTopologyTestSuite runs all tests in this test suite
func TestStatefulTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(StatefulTopologyTestSuite))
}

func (suite *StatefulTopologyTestSuite) SetupTest() {
	suite.histFlag = true
}

func (suite *StatefulTopologyTestSuite) TestSingleSystemLowScale() {
	suite.systemScenario(10, 10, 100, 120, 5, 100, 4)
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

	collector, _ := test.GenerateIDs(suite.T(), col, !test.DryRun, unittest.WithRole(flow.RoleCollection))
	access, _ := test.GenerateIDs(suite.T(), acc, !test.DryRun, unittest.WithRole(flow.RoleAccess))
	consensus, _ := test.GenerateIDs(suite.T(), con, !test.DryRun, unittest.WithRole(flow.RoleConsensus))
	verification, _ := test.GenerateIDs(suite.T(), ver, !test.DryRun, unittest.WithRole(flow.RoleVerification))
	execution, _ := test.GenerateIDs(suite.T(), exe, !test.DryRun, unittest.WithRole(flow.RoleExecution))

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

// systemScenario is a test helper evaluates that generates several systems with specfies number of nodes
// on each role. It then evaluates that sub-fanout of each role in topology is
// bound by the fanout function of topology on the role's entire size.
func (suite *StatefulTopologyTestSuite) systemScenario(system, acc, col, con, exe, ver, cluster int) {
	// creates a histogram to keep average fanout of nodes in systems
	aveHist := thist.NewHist(nil, fmt.Sprintf("Average fanout for %d systems", system),
		"fit", 10, false)

	for j := 0; j < system; j++ {
		// creates a flow system
		state, ids, subMngrs := suite.generateSystem(acc, col, con, exe, ver, cluster)

		// creates a fanout histogram for this system
		systemHist := thist.NewHist(nil, fmt.Sprintf("System #%d fanout", j),
			"auto", -1, false)

		totalFanout := 0 // keeps summation of nodes' fanout for statistical reason

		// creates topology and topology manager
		for i, id := range ids {
			fanout := suite.subFanoutScenario(id.NodeID, subMngrs[i], ids, state)
			systemHist.Update(float64(fanout))
			totalFanout += fanout
		}

		if suite.histFlag {
			// prints fanout histogram of this system
			fmt.Println(systemHist.Draw())
		}

		// keeps track of average fanout per node
		aveHist.Update(float64(totalFanout) / float64(len(ids)))
	}

	if suite.histFlag {
		fmt.Println(aveHist.Draw())
	}
}

// subFanoutScenario is a test helper that creates a stateful topology manager with a linear fanout function for the
// node. It then evaluates that sub-fanout of each role in topology is
// bound by the fanout function of topology on the role's entire size.
// For example if fanout function is `n+1/2` and we have x consensus nodes,
// then this tests evaluates that no node should have more than or equal to `x+1/2`
// consensus fanout.
//
// It returns the fanout of node.
func (suite *StatefulTopologyTestSuite) subFanoutScenario(me flow.Identifier,
	subMngr channel.SubscriptionManager,
	ids flow.IdentityList,
	state protocol.ReadOnlyState) int {
	// creates a graph sampler for the node
	graphSampler, err := topology.NewLinearFanoutGraphSampler(me)
	require.NoError(suite.T(), err)

	// creates topology of the node
	top, err := topology.NewTopicBasedTopology(me, state, graphSampler)
	require.NoError(suite.T(), err)

	// creates topology manager
	topMngr := topology.NewStatefulTopologyManager(top, subMngr)

	// generates topology of node
	myFanout, err := topMngr.MakeTopology(ids)
	require.GreaterOrEqual(suite.T(), uint(len(myFanout)), topology.LinearFanoutFunc(uint(len(ids))))
	require.NoError(suite.T(), err)

	for _, role := range flow.Roles() {
		// total number of nodes in flow with specified role
		roleTotal := uint(len(ids.Filter(filter.HasRole(role))))
		// number of nodes in fanout with specified role
		roleFanout := topology.LinearFanoutFunc(uint(len(myFanout.Filter(filter.HasRole(role)))))
		require.Less(suite.T(), roleFanout, roleTotal)
	}

	return len(myFanout)
}
