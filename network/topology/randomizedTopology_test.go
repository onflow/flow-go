package topology

import (
	"os"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// RandomizedTopologyTestSuite tests encapsulates tests around the randomized topology.
type RandomizedTopologyTestSuite struct {
	suite.Suite
	state    protocol.State    // represents a mocked protocol state
	all      flow.IdentityList // represents the identity list of all nodes in the system
	clusters flow.ClusterList  // represents list of cluster ids of collection nodes
	logger   zerolog.Logger
	edgeProb float64
}

// TestRandomizedTopologyTestSuite starts all the tests in this test suite.
func TestRandomizedTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(RandomizedTopologyTestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *RandomizedTopologyTestSuite) SetupTest() {
	// generates 1000 nodes including 100 collection nodes in 3 clusters.
	nClusters := 3
	nCollectors := 100
	nTotal := 1000

	suite.logger = zerolog.New(os.Stderr).Level(zerolog.DebugLevel)

	// sets edge probability to 0.3
	suite.edgeProb = 0.3

	collectors := unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	others := unittest.IdentityListFixture(nTotal, unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.all = append(others, collectors...)

	// mocks state for collector nodes topology
	suite.state, suite.clusters = MockStateForCollectionNodes(suite.T(),
		suite.all.Filter(filter.HasRole(flow.RoleCollection)), uint(nClusters))
}

// TestUnhappyInitialization concerns initializing randomized topology with unhappy inputs.
func (suite *RandomizedTopologyTestSuite) TestUnhappyInitialization() {
	// initializing with zero edge probability should return an err
	_, err := NewRandomizedTopology(suite.all[0].NodeID, suite.logger, 0, suite.state)
	require.Error(suite.T(), err)

	// initializing with negative edge probability should return an err
	_, err = NewRandomizedTopology(suite.all[0].NodeID, suite.logger, -0.5, suite.state)
	require.Error(suite.T(), err)

	// initializing with zero edge probability below 0.01 should return an err
	_, err = NewRandomizedTopology(suite.all[0].NodeID, suite.logger, 0.009, suite.state)
	require.Error(suite.T(), err)

	// initializing with zero edge probability above 1 should return an err
	_, err = NewRandomizedTopology(suite.all[0].NodeID, suite.logger, 1.0001, suite.state)
	require.Error(suite.T(), err)
}

// TestUniqueness is a weak uniqueness check evaluates that no two consecutive connected components
// on the same channel are the same.
// It generates a topology for the first topic of consensus nodes.
// Since topologies are seeded with the node ids, it evaluates that every two consecutive
// topologies of the same topic for distinct nodes are distinct.
//
// It also checks the topology against non-inclusion of the node itself in its own topology.
//
// Note: currently we are using a uniform edge probability, hence there are
// 2^(n * (n-1)) many unique topologies for the same topic across different nodes.
// Even for small numbers like n = 10, the potential outcomes are large enough (i.e., 10e30) so that the uniqueness is guaranteed.
// This test however, performs a very weak uniqueness test by checking the uniqueness among consecutive topologies.
//
// With the edge probability in place, it is also very unlikely that any two arbitrary nodes end up the same
// size topology.
func (suite *RandomizedTopologyTestSuite) TestUniqueness() {
	var previous, current []string

	topics := engine.ChannelsByRole(flow.RoleConsensus)
	require.Greater(suite.T(), len(topics), 1)

	for _, identity := range suite.all {
		if identity.Role != flow.RoleConsensus {
			continue
		}

		previous = current
		current = nil

		// creates and samples a new topic aware topology for the first topic of consensus nodes
		top, err := NewRandomizedTopology(identity.NodeID, suite.logger, suite.edgeProb, suite.state)
		require.NoError(suite.T(), err)
		ids, err := top.subsetChannel(suite.all, topics[0])
		require.NoError(suite.T(), err)

		// topology should not contain the node itself
		require.Empty(suite.T(), ids.Filter(filter.HasNodeID(identity.NodeID)))

		for _, v := range ids {
			current = append(current, v.NodeID.String())
		}
		sort.Strings(current)

		if previous == nil {
			continue
		}

		// assert a different seed generates a different topology
		require.NotEqual(suite.T(), previous, current)
	}
}

// TestConnectedness_NonClusterChannel checks whether graph components corresponding to a
// non-cluster channel are individually connected.
func (suite *RandomizedTopologyTestSuite) TestConnectedness_NonClusterChannel() {
	channel := engine.TestNetwork
	// adjacency map keeps graph component of a single channel
	channelAdjMap := make(map[flow.Identifier]flow.IdentityList)

	for _, id := range suite.all {
		// creates a topic-based topology for node
		top, err := NewRandomizedTopology(id.NodeID, suite.logger, suite.edgeProb, suite.state)
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.subsetChannel(suite.all, channel)
		require.NoError(suite.T(), err)

		uniquenessCheck(suite.T(), subset)

		channelAdjMap[id.NodeID] = subset
	}

	connectednessByChannel(suite.T(), channelAdjMap, suite.all, channel)
}

// TestConnectedness_NonClusterChannel checks whether graph components corresponding to a
// cluster channel are individually connected.
func (suite *RandomizedTopologyTestSuite) TestConnectedness_ClusterChannel() {
	// picks one cluster channel as sample
	channel := engine.ChannelSyncCluster(unittest.BlockFixture().Header.ChainID)

	// adjacency map keeps graph component of a single channel
	channelAdjMap := make(map[flow.Identifier]flow.IdentityList)

	// iterates over collection nodes
	for _, id := range suite.all.Filter(filter.HasRole(flow.RoleCollection)) {
		// creates a randomized topology for node
		top, err := NewRandomizedTopology(id.NodeID, suite.logger, suite.edgeProb, suite.state)
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.subsetChannel(suite.all, channel)
		require.NoError(suite.T(), err)

		uniquenessCheck(suite.T(), subset)

		channelAdjMap[id.NodeID] = subset
	}

	for _, cluster := range suite.clusters {
		connectedByCluster(suite.T(), channelAdjMap, suite.all, cluster)
	}
}

// TestSampleFanout_EmptyAllSet evaluates that trying to sample a connected graph when `all`
// is empty returns an error.
func (suite *RandomizedTopologyTestSuite) TestSampleFanout_EmptyAllSet() {
	// creates a topology for the node
	top, err := NewRandomizedTopology(suite.all[0].NodeID, suite.logger, suite.edgeProb, suite.state)
	require.NoError(suite.T(), err)

	// sampling with empty `all`
	_, err = top.sampleFanout(flow.IdentityList{})
	require.Error(suite.T(), err)

	// sampling with nil all
	_, err = top.sampleFanout(nil)
	require.Error(suite.T(), err)
}

// TestSampleFanoutConnectedness evaluates that sampleFanout with default edge probability of this suite provides
// a connected graph.
func (suite *RandomizedTopologyTestSuite) TestSampleFanoutConnectedness() {
	adjMap := make(map[flow.Identifier]flow.IdentityList)
	for _, id := range suite.all {
		// creates a topology for the node
		top, err := NewRandomizedTopology(id.NodeID, suite.logger, suite.edgeProb, suite.state)
		require.NoError(suite.T(), err)

		// samples a graph and stores it in adjacency map
		sample, err := top.sampleFanout(suite.all)
		require.NoError(suite.T(), err)

		uniquenessCheck(suite.T(), sample)

		adjMap[id.NodeID] = sample
	}

	Connected(suite.T(), adjMap, suite.all, filter.Any)
}
