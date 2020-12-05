package topology_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/bsipos/thist"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/test"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

type TopologyConstructorFunc func(flow.Identifier, protocol.ReadOnlyState, network.SubscriptionManager) (network.Topology, error)

// TopologyTestSuite tests the end-to-end connectedness of topology
type TopologyTestSuite struct {
	suite.Suite
	logger          zerolog.Logger
	linearFanoutTop TopologyConstructorFunc
	randomizedTop   TopologyConstructorFunc
}

// TestTopologyTestSuite runs all tests in this test suite
func TestTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(TopologyTestSuite))
}

// SetupTest is executed prior to every test in this test suite.
func (suite *TopologyTestSuite) SetupTest() {
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	suite.linearFanoutTop = func(identifier flow.Identifier, state protocol.ReadOnlyState, manager network.SubscriptionManager) (network.Topology,
		error) {

		return topology.NewTopicBasedTopology(identifier, suite.logger, state, manager)
	}

	suite.randomizedTop = func(identifier flow.Identifier, state protocol.ReadOnlyState, manager network.SubscriptionManager) (network.Topology,
		error) {
		return topology.NewRandomizedTopology(identifier, 0.01, state, manager)
	}
}

// TestLowScaleLinearFanout creates systems with
// 10 access nodes
// 100 collection nodes in 4 clusters
// 120 consensus nodes
// 5 execution nodes
// 100 verification nodes
// and builds a stateful linear fanout topology for the systems.
// For each system, it then checks the end-to-end connectedness of the topology graph.
func (suite *TopologyTestSuite) TestLowScaleLinearFanout() {
	suite.multiSystemEndToEndConnectedness(suite.linearFanoutTop, 1, 10, 100, 120, 5, 100, 4)
}

// TestLowScaleRandomized creates systems with
// 10 access nodes
// 100 collection nodes in 4 clusters
// 120 consensus nodes
// 5 execution nodes
// 100 verification nodes
// and builds a randomized topology for the systems.
// For each system, it then checks the end-to-end connectedness of the topology graph.
func (suite *TopologyTestSuite) TestLowScaleRandomized() {
	suite.multiSystemEndToEndConnectedness(suite.randomizedTop, 200, 10, 100, 120, 5, 100, 4)
}

// TestModerateScaleLinearFanout creates systems with
// 20 access nodes
// 200 collection nodes in 8 clusters
// 240 consensus nodes
// 10 execution nodes
// 100 verification nodes
// and builds a stateful linear fanout topology for the systems.
// For each system, it then checks the end-to-end connectedness of the topology graph.
func (suite *TopologyTestSuite) TestModerateScaleLinearFanout() {
	suite.multiSystemEndToEndConnectedness(suite.linearFanoutTop, 1, 20, 200, 240, 10, 200, 8)
}

// TestModerateScaleRandomized creates systems with
// 20 access nodes
// 200 collection nodes in 8 clusters
// 240 consensus nodes
// 10 execution nodes
// 100 verification nodes
// and builds a stateful randomized topology for the systems.
// For each system, it then checks the end-to-end connectedness of the topology graph.
func (suite *TopologyTestSuite) TestModerateScale() {
	suite.multiSystemEndToEndConnectedness(suite.randomizedTop, 1, 20, 200, 240, 10, 200, 8)
}

// TestHighScaleLinearFanout creates systems with
// 40 access nodes
// 400 collection nodes in 16 clusters
// 480 consensus nodes
// 20 execution nodes
// 400 verification nodes
// and builds a stateful topology for the systems.
// For each system, it then checks the end-to-end connectedness of the topology graph.
func (suite *TopologyTestSuite) TestHighScaleLinearFanout() {
	suite.multiSystemEndToEndConnectedness(suite.linearFanoutTop, 1, 40, 400, 480, 20, 400, 16)
}

// TestHighScaleRandomized creates systems with
// 40 access nodes
// 400 collection nodes in 16 clusters
// 480 consensus nodes
// 20 execution nodes
// 400 verification nodes
// and builds a stateful randomized topology for the systems.
// For each system, it then checks the end-to-end connectedness of the topology graph.
func (suite *TopologyTestSuite) TestHighScaleRandomized() {
	suite.multiSystemEndToEndConnectedness(suite.randomizedTop, 1, 40, 400, 480, 20, 400, 16)
}

// generateSystem is a test helper that given number of nodes per role as well as desire number of clusters
// generates the protocol state, identity list and subscription managers for the nodes.
// - acc: number of access nodes
// - col: number of collection nodes
// - exe: number of execution nodes
// - ver: number of verification nodes
// - cluster: number of clusters of collection nodes
func (suite *TopologyTestSuite) generateSystem(acc, col, con, exe, ver, cluster int) (protocol.State,
	flow.IdentityList,
	[]network.SubscriptionManager) {

	collector, _ := test.GenerateIDs(suite.T(), suite.logger, col, test.DryRun, unittest.WithRole(flow.RoleCollection))
	access, _ := test.GenerateIDs(suite.T(), suite.logger, acc, test.DryRun, unittest.WithRole(flow.RoleAccess))
	consensus, _ := test.GenerateIDs(suite.T(), suite.logger, con, test.DryRun, unittest.WithRole(flow.RoleConsensus))
	verification, _ := test.GenerateIDs(suite.T(), suite.logger, ver, test.DryRun, unittest.WithRole(flow.RoleVerification))
	execution, _ := test.GenerateIDs(suite.T(), suite.logger, exe, test.DryRun, unittest.WithRole(flow.RoleExecution))

	ids := flow.IdentityList{}
	ids = ids.Union(collector)
	ids = ids.Union(access)
	ids = ids.Union(consensus)
	ids = ids.Union(verification)
	ids = ids.Union(execution)

	// mocks state for collector nodes topology
	state, _ := topology.CreateMockStateForCollectionNodes(suite.T(),
		ids.Filter(filter.HasRole(flow.RoleCollection)), uint(cluster))

	subMngrs := topology.MockSubscriptionManager(suite.T(), ids)

	return state, ids, subMngrs
}

// multiSystemEndToEndConnectedness is a test helper evaluates end-to-end connectedness of the system graph
// over several number of systems each with specified number of nodes on each role.
func (suite *TopologyTestSuite) multiSystemEndToEndConnectedness(constructorFunc TopologyConstructorFunc, system, acc, col, con, exe, ver,
	cluster int) {
	// creates a histogram to keep average fanout of nodes in systems
	var aveHist *thist.Hist
	if suite.printTrace() {
		aveHist = thist.NewHist(nil, fmt.Sprintf("Average fanout for %d systems", system), "fit", 10, false)
	}

	for j := 0; j < system; j++ {
		// adjacency map keeps graph component of a single channel ID
		adjMap := make(map[flow.Identifier]flow.IdentityList)

		// creates a flow system
		state, ids, subMngrs := suite.generateSystem(acc, col, con, exe, ver, cluster)

		var systemHist *thist.Hist
		if suite.printTrace() {
			// creates a fanout histogram for this system
			systemHist = thist.NewHist(nil, fmt.Sprintf("System #%d fanout", j), "auto", -1, false)
		}

		totalFanout := 0 // keeps summation of nodes' fanout for statistical reason

		// creates topology of the nodes
		for i, id := range ids {
			fanout := suite.topologyScenario(constructorFunc, id.NodeID, subMngrs[i], ids, state)

			adjMap[id.NodeID] = fanout

			if suite.printTrace() {
				systemHist.Update(float64(len(fanout)))
			}
			totalFanout += len(fanout)
		}

		if suite.printTrace() {
			// prints fanout histogram of this system
			fmt.Println(systemHist.Draw())
			// keeps track of average fanout per node
			aveHist.Update(float64(totalFanout) / float64(len(ids)))
		}

		// checks end-to-end connectedness of the topology
		topology.CheckConnectedness(suite.T(), adjMap, ids)
	}

	if suite.printTrace() {
		fmt.Println(aveHist.Draw())
	}
}

// topologyScenario is a test helper that creates a StatefulTopologyManager with the LinearFanoutFunc,
// it creates a TopicBasedTopology for the node and returns its fanout.
func (suite *TopologyTestSuite) topologyScenario(constructorFunc TopologyConstructorFunc, me flow.Identifier,
	subMngr network.SubscriptionManager,
	ids flow.IdentityList,
	state protocol.ReadOnlyState) flow.IdentityList {

	// creates topology of the node
	top, err := constructorFunc(me, state, subMngr)
	require.NoError(suite.T(), err)

	// generates topology of node
	myFanout, err := top.GenerateFanout(ids)
	require.NoError(suite.T(), err)

	return myFanout
}

// printTrace returns true if local environment variable Trace is found.
func (suite *TopologyTestSuite) printTrace() bool {
	//_, found := os.LookupEnv("Trace")
	//return found
	return true
}
