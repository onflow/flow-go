package topology

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/channel"
	protocol2 "github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// RandomizedTopologyTestSuite tests the bare minimum requirements of a
// topology that is needed for our network.
type RandomizedTopologyTestSuite struct {
	suite.Suite
	state    protocol2.State   // represents a mocked protocol state
	all      flow.IdentityList // represents the identity list of all nodes in the system
	clusters flow.ClusterList  // represents list of cluster ids of collection nodes
	subMngr  []channel.SubscriptionManager
	edgeProb float64
}

// TestRandomizedTopologyTestSuite starts all the tests in this test suite
func TestRandomizedTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(RandomizedTopologyTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (suite *RandomizedTopologyTestSuite) SetupTest() {
	// generates 1000 nodes including 100 collection nodes in 3 clusters.
	nClusters := 3
	nCollectors := 100
	nTotal := 1000

	// sets edge probability to 0.3
	suite.edgeProb = 0.3

	collectors := unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	others := unittest.IdentityListFixture(nTotal, unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.all = append(others, collectors...)

	// mocks state for collector nodes topology
	suite.state, suite.clusters = CreateMockStateForCollectionNodes(suite.T(),
		suite.all.Filter(filter.HasRole(flow.RoleCollection)), uint(nClusters))

	suite.subMngr = MockSubscriptionManager(suite.T(), suite.all)
}

// TestUnhappyInitialization concerns initializing randomized topology with unhappy inputs
func (suite *RandomizedTopologyTestSuite) TestUnhappyInitialization() {
	// initializing with zero edge probability should return an err
	_, err := NewRandomizedTopology(suite.all[0].NodeID, 0, suite.state, suite.subMngr[0])
	require.Error(suite.T(), err)

	// initializing with negative edge probability should return an err
	_, err = NewRandomizedTopology(suite.all[0].NodeID, -0.5, suite.state, suite.subMngr[0])
	require.Error(suite.T(), err)

	// initializing with zero edge probability below 0.01 should return an err
	_, err = NewRandomizedTopology(suite.all[0].NodeID, 0.009, suite.state, suite.subMngr[0])
	require.Error(suite.T(), err)

	// initializing with zero edge probability above 1 should return an err
	_, err = NewRandomizedTopology(suite.all[0].NodeID, 1.0001, suite.state, suite.subMngr[0])
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
// Note: currently we are using a uniform probability, hence there are
// C(n, (n+1)/2) many unique topologies for the same topic across different nodes. Even for small numbers
// like n = 300, the potential outcomes are large enough (i.e., 10e88) so that the uniqueness is guaranteed.
// This test however, performs a very weak uniqueness test by checking the uniqueness among consecutive topologies.
func (suite *RandomizedTopologyTestSuite) TestUniqueness() {
	var previous, current []string

	// for each topic samples 100 topologies
	// all topologies for a topic should be the same
	topics := engine.ChannelIDsByRole(flow.RoleConsensus)
	require.Greater(suite.T(), len(topics), 1)

	for i, identity := range suite.all {
		// extracts all topics node (i) subscribed to
		if identity.Role != flow.RoleConsensus {
			continue
		}

		previous = current
		current = nil

		// creates and samples a new topic aware topology for the first topic of consensus nodes
		top, err := NewRandomizedTopology(identity.NodeID, suite.edgeProb, suite.state, suite.subMngr[i])
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

		// assert that a different seed generates a different topology
		require.NotEqual(suite.T(), previous, current)
	}
}

// TestConnectedness_NonClusterChannelID checks whether graph components corresponding to a
// non-cluster channel ID are individually connected.
func (suite *RandomizedTopologyTestSuite) TestConnectedness_NonClusterChannelID() {
	channelID := engine.TestNetwork
	// adjacency map keeps graph component of a single channel ID
	channelIDAdjMap := make(map[flow.Identifier]flow.IdentityList)

	for i, id := range suite.all {
		// creates a topic-based topology for node
		top, err := NewRandomizedTopology(id.NodeID, suite.edgeProb, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.subsetChannel(suite.all, channelID)
		require.NoError(suite.T(), err)

		channelIDAdjMap[id.NodeID] = subset
	}

	CheckConnectednessByChannelID(suite.T(), channelIDAdjMap, suite.all, channelID)
}

// TestConnectedness_NonClusterChannelID checks whether graph components corresponding to a
// cluster channel ID are individually connected.
func (suite *RandomizedTopologyTestSuite) TestConnectedness_ClusterChannelID() {
	// picks one cluster channel ID as sample
	channelID := clusterChannelIDs(suite.T())[0]

	// adjacency map keeps graph component of a single channel ID
	channelIDAdjMap := make(map[flow.Identifier]flow.IdentityList)

	// iterates over collection nodes
	for i, id := range suite.all.Filter(filter.HasRole(flow.RoleCollection)) {
		// creates a randomized topology for node
		top, err := NewRandomizedTopology(id.NodeID, suite.edgeProb, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.subsetChannel(suite.all, channelID)
		require.NoError(suite.T(), err)

		uniquenessCheck(suite.T(), subset)

		channelIDAdjMap[id.NodeID] = subset
	}

	// check that each of the collection clusters forms a connected graph
	for _, cluster := range suite.clusters {
		checkConnectednessByCluster(suite.T(), channelIDAdjMap, suite.all, cluster)
	}
}

// TestLinearFanout_EmptyAllSet evaluates that trying to sample a connected graph when `all`
// is empty returns an error.
func (suite *RandomizedTopologyTestSuite) TestLinearFanout_EmptyAllSet() {
	// creates a topology for the node
	top, err := NewRandomizedTopology(suite.all[0].NodeID, suite.edgeProb, suite.state, suite.subMngr[0])
	require.NoError(suite.T(), err)

	// sampling with empty `all`
	_, err = top.sampleFanout(flow.IdentityList{})
	require.Error(suite.T(), err)

	// sampling with empty all
	_, err = top.sampleFanout(flow.IdentityList{})
	require.Error(suite.T(), err)

	// sampling with nil all
	_, err = top.sampleFanout(nil)
	require.Error(suite.T(), err)
}

// TestSampleFanoutConnectedness evaluates that sampleFanout with default edge probability of this suite provides
// a connected graph,
func (suite *RandomizedTopologyTestSuite) TestSampleFanoutConnectedness() {
	adjMap := make(map[flow.Identifier]flow.IdentityList)
	for i, id := range suite.all {
		// creates a topology for the node
		top, err := NewRandomizedTopology(id.NodeID, suite.edgeProb, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)

		// samples a graph and stores it in adjacency map
		sample, err := top.sampleFanout(suite.all)
		require.NoError(suite.T(), err)

		uniquenessCheck(suite.T(), sample)

		adjMap[id.NodeID] = sample
	}

	CheckGraphConnected(suite.T(), adjMap, suite.all, filter.In(suite.all))
}
