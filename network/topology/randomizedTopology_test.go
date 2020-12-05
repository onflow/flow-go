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
	"github.com/onflow/flow-go/network"
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
	subMngr  []network.SubscriptionManager
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

	collectors := unittest.IdentityListFixture(nCollectors, unittest.WithRole(flow.RoleCollection))
	others := unittest.IdentityListFixture(nTotal, unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.all = append(others, collectors...)

	// mocks state for collector nodes topology
	suite.state, suite.clusters = CreateMockStateForCollectionNodes(suite.T(),
		suite.all.Filter(filter.HasRole(flow.RoleCollection)), uint(nClusters))

	suite.subMngr = MockSubscriptionManager(suite.T(), suite.all)

	suite.logger = zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
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

// TestTopologySize_Topic verifies that size of each topology fanout per topic is greater than
// `(k+1)/2` where `k` is number of nodes subscribed to a topic. It does that over 100 random iterations.
func (suite *RandomizedTopologyTestSuite) TestTopologySize_Topic() {
	for i := 0; i < 100; i++ {
		top, err := NewTopicBasedTopology(suite.all[0].NodeID, suite.state, suite.subMngr[0])
		require.NoError(suite.T(), err)

		topics := engine.ChannelIDsByRole(suite.all[0].Role)
		require.Greater(suite.T(), len(topics), 1)

		for _, topic := range topics {
			// extracts total number of nodes subscribed to topic
			roles, ok := engine.RolesByChannelID(topic)
			require.True(suite.T(), ok)

			ids, err := top.subsetChannel(suite.all, nil, topic)
			require.NoError(suite.T(), err)

			// counts total number of nodes that has the roles and are not `suite.me`  (node of interest).
			total := len(suite.all.Filter(filter.And(filter.HasRole(roles...),
				filter.Not(filter.HasNodeID(suite.all[0].NodeID)))))
			require.True(suite.T(), float64(len(ids)) >= (float64)(total+1)/2)
		}
	}
}

// TestDeteministicity is a weak test that verifies the same seed generates the same topology for a topic.
//
// It also checks the topology against non-inclusion of the node itself in its own topology.
func (suite *RandomizedTopologyTestSuite) TestDeteministicity() {
	// creates a topology using the graph sampler
	top, err := NewTopicBasedTopology(suite.all[0].NodeID, suite.state, suite.subMngr[0])
	require.NoError(suite.T(), err)

	topics := engine.ChannelIDsByRole(suite.all[0].Role)
	require.Greater(suite.T(), len(topics), 1)

	// for each topic samples 100 topologies
	// all topologies for a topic should be the same
	for _, topic := range topics {
		var previous, current []string
		for i := 0; i < 100; i++ {
			previous = current
			current = nil

			// generate a new topology with a the same all, size and seed
			ids, err := top.subsetChannel(suite.all, nil, topic)
			require.NoError(suite.T(), err)

			// topology should not contain the node itself
			require.Empty(suite.T(), ids.Filter(filter.HasNodeID(suite.all[0].NodeID)))

			for _, v := range ids {
				current = append(current, v.NodeID.String())
			}
			// no guarantees about order is made by Topology.subsetChannel(), hence sort the return values before comparision
			sort.Strings(current)

			if previous == nil {
				continue
			}

			// assert that a different seed generates a different topology
			require.Equal(suite.T(), previous, current)
		}
	}
}

// TestUniqueness generates a topology for the first topic of consensus nodes.
// Since topologies are seeded with the node ids, it evaluates that every two consecutive
// topologies of the same topic for distinct nodes are distinct.
//
// It also checks the topology against non-inclusion of the node itself in its own topology.
//
// Note: currently we are using a linear fanout for guaranteed delivery, hence there are
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
		top, err := NewTopicBasedTopology(identity.NodeID, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)
		ids, err := top.subsetChannel(suite.all, nil, topics[0])
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

// TestConnectedness_NonClusterTopics checks whether graph components corresponding to a
// non-cluster channel ID are individually connected.
func (suite *RandomizedTopologyTestSuite) TestConnectedness_NonClusterChannelID() {
	channelID := engine.TestNetwork
	// adjacency map keeps graph component of a single channel ID
	channelIDAdjMap := make(map[flow.Identifier]flow.IdentityList)

	for i, id := range suite.all {
		// creates a topic-based topology for node
		top, err := NewTopicBasedTopology(id.NodeID, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.subsetChannel(suite.all, nil, channelID)
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
		// creates a channelID-based topology for node
		top, err := NewTopicBasedTopology(id.NodeID, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.subsetChannel(suite.all, nil, channelID)
		require.NoError(suite.T(), err)

		channelIDAdjMap[id.NodeID] = subset
	}

	// check that each of the collection clusters forms a connected graph
	for _, cluster := range suite.clusters {
		suite.checkConnectednessByCluster(suite.T(), channelIDAdjMap, cluster)
	}
}

// TestLinearFanout_UnconditionalSampling evaluates that sampling a connected graph fanout
// with an empty `shouldHave` list follows the LinearFanoutFunc,
// and it also does not contain duplicate element.
func (suite *RandomizedTopologyTestSuite) TestLinearFanout_UnconditionalSampling() {
	// samples with no `shouldHave` set.
	top, err := NewTopicBasedTopology(suite.all[0].NodeID, suite.state, suite.subMngr[0])
	require.NoError(suite.T(), err)

	sample, err := top.sampleConnectedGraph(suite.all, nil)
	require.NoError(suite.T(), err)

	// the LinearFanoutGraphSampler utilizes the LinearFanoutFunc. Hence any sample it makes should have
	// the size equal to applying LinearFanoutFunc over the original set.
	expectedFanout := LinearFanoutFunc(len(suite.all))
	require.Equal(suite.T(), len(sample), expectedFanout)

	// checks sample does not include any duplicate
	suite.uniquenessCheck(sample)
}

// TestLinearFanout_ConditionalSampling evaluates that sampling a connected graph fanout with a shouldHave set
// follows the LinearFanoutFunc, and it also does not contain duplicate element.
func (suite *RandomizedTopologyTestSuite) TestLinearFanout_ConditionalSampling() {
	// samples 10 all into `shouldHave` set.
	shouldHave := suite.all.Sample(10)

	// creates a topology for the node
	top, err := NewTopicBasedTopology(suite.all[0].NodeID, suite.state, suite.subMngr[0])
	require.NoError(suite.T(), err)

	// samples a connected graph of `all` that includes `shouldHave` set.
	sample, err := top.sampleConnectedGraph(suite.all, shouldHave)
	require.NoError(suite.T(), err)

	// the LinearFanoutGraphSampler utilizes the LinearFanoutFunc. Hence any sample it makes should have
	// the size equal to applying LinearFanoutFunc over the original set.
	expectedFanout := LinearFanoutFunc(len(suite.all))
	require.Equal(suite.T(), len(sample), expectedFanout)

	// checks sample does not include any duplicate
	suite.uniquenessCheck(sample)

	// checks inclusion of all shouldHave ones into sample
	for _, id := range shouldHave {
		require.Contains(suite.T(), sample, id)
	}
}

// TestLinearFanout_SmallerAll evaluates that sampling a connected graph fanout with a shouldHave set
// that is greater than required fanout, returns the `shouldHave` set instead.
func (suite *RandomizedTopologyTestSuite) TestLinearFanout_SmallerAll() {
	// samples 10 all into 'shouldHave'.
	shouldHave := suite.all.Sample(10)
	// samples a smaller component of all with 5 nodes and combines with `shouldHave`
	smallerAll := suite.all.Filter(filter.Not(filter.In(shouldHave))).Sample(5).Union(shouldHave)

	// creates a topology for the node
	top, err := NewTopicBasedTopology(suite.all[0].NodeID, suite.state, suite.subMngr[0])
	require.NoError(suite.T(), err)

	// total size of smallerAll is 15, and it requires a linear fanout of 8 which is less than
	// size of `shouldHave` set, so the shouldHave itself should return
	sample, err := top.sampleConnectedGraph(smallerAll, shouldHave)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), len(sample), len(shouldHave))
	require.ElementsMatch(suite.T(), sample, shouldHave)
}

// TestLinearFanout_SubsetViolence evaluates that trying to sample a connected graph when `shouldHave`
// is not a subset of `all` returns an error.
func (suite *RandomizedTopologyTestSuite) TestLinearFanout_SubsetViolence() {
	// samples 10 all into 'shouldHave',
	shouldHave := suite.all.Sample(10)
	// excludes one of the `shouldHave` all from all, hence it is no longer a subset
	excludedAll := suite.all.Filter(filter.Not(filter.HasNodeID(shouldHave[0].NodeID)))

	// creates a topology for the node
	top, err := NewTopicBasedTopology(suite.all[0].NodeID, suite.state, suite.subMngr[0])
	require.NoError(suite.T(), err)

	// since `shouldHave` is not a subset of `excludedAll` it should return an error
	_, err = top.sampleConnectedGraph(excludedAll, shouldHave)
	require.Error(suite.T(), err)
}

// TestLinearFanout_EmptyAllSet evaluates that trying to sample a connected graph when `all`
// is empty returns an error.
func (suite *RandomizedTopologyTestSuite) TestLinearFanout_EmptyAllSet() {
	// samples 10 all into 'shouldHave'.
	shouldHave := suite.all.Sample(10)

	// creates a topology for the node
	top, err := NewTopicBasedTopology(suite.all[0].NodeID, suite.state, suite.subMngr[0])
	require.NoError(suite.T(), err)

	// sampling with empty `all` and non-empty `shouldHave`
	_, err = top.sampleConnectedGraph(flow.IdentityList{}, shouldHave)
	require.Error(suite.T(), err)

	// sampling with empty all and nil `shouldHave`
	_, err = top.sampleConnectedGraph(flow.IdentityList{}, nil)
	require.Error(suite.T(), err)

	// sampling with nil all and nil `shouldHave`
	_, err = top.sampleConnectedGraph(nil, nil)
	require.Error(suite.T(), err)
}

// TestConnectedness_Unconditionally evaluates that samples returned by the LinearFanoutGraphSampler with
// empty `shouldHave` constitute a connected graph.
func (suite *RandomizedTopologyTestSuite) TestConnectedness_Unconditionally() {
	adjMap := make(map[flow.Identifier]flow.IdentityList)
	for i, id := range suite.all {
		// creates a topology for the node
		top, err := NewTopicBasedTopology(id.NodeID, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)

		// samples a graph and stores it in adjacency map
		sample, err := top.sampleConnectedGraph(suite.all, nil)
		adjMap[id.NodeID] = sample
	}

	CheckGraphConnected(suite.T(), adjMap, suite.all, filter.In(suite.all))
}

// TestConnectedness_Conditionally evaluates that samples returned by the LinearFanoutGraphSampler with
// some `shouldHave` constitute a connected graph.
func (suite *RandomizedTopologyTestSuite) TestConnectedness_Conditionally() {
	adjMap := make(map[flow.Identifier]flow.IdentityList)
	for i, id := range suite.all {
		// creates a topology for the node
		top, err := NewTopicBasedTopology(id.NodeID, suite.state, suite.subMngr[i])
		require.NoError(suite.T(), err)

		// samples a graph and stores it in adjacency map
		// sampling is done with a non-empty `shouldHave` subset of 10 randomly chosen all
		shouldHave := suite.all.Sample(10)
		sample, err := top.sampleConnectedGraph(suite.all, shouldHave)

		// evaluates inclusion of should haves in sample
		for _, shouldHaveID := range shouldHave {
			require.Contains(suite.T(), sample, shouldHaveID)
		}
		adjMap[id.NodeID] = sample
	}

	CheckGraphConnected(suite.T(), adjMap, suite.all, filter.In(suite.all))
}

// uniquenessCheck is a test helper method that fails the test if all include any duplicate identity.
func (suite *RandomizedTopologyTestSuite) uniquenessCheck(ids flow.IdentityList) {
	seen := make(map[flow.Identity]struct{})
	for _, id := range ids {
		// checks if id is duplicate in ids list
		_, ok := seen[*id]
		require.False(suite.T(), ok)

		// marks id as seen
		seen[*id] = struct{}{}
	}
}

// checkConnectednessByCluster is a test helper that checks all nodes belong to a cluster are connected.
func (suite *RandomizedTopologyTestSuite) checkConnectednessByCluster(t *testing.T,
	adjMap map[flow.Identifier]flow.IdentityList,
	cluster flow.IdentityList) {
	CheckGraphConnected(t,
		adjMap,
		suite.all,
		filter.In(cluster))
}
