package topology_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/network/gossip/libp2p/test"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	protocol2 "github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// TopicAwareTopologyTestSuite tests the bare minimum requirements of a
// topology that is needed for our network. It should not replace the information
// theory assumptions behind the schemes, e.g., random oracle model of hashes
type TopicAwareTopologyTestSuite struct {
	suite.Suite
	state    protocol2.State   // represents a mocked protocol state
	ids      flow.IdentityList // represents the identity list of all nodes in the system
	me       flow.Identity     // represents identity of single instance of node that creates topology
	clusters flow.ClusterList  // represents list of cluster ids of collection nodes
	fanout   uint              // represents maximum number of connections this peer allows to have
}

// TestTopicAwareTopologyTestSuite starts all the tests in this test suite
func TestTopicAwareTopologyTestSuite(t *testing.T) {
	suite.Run(t, new(TopicAwareTopologyTestSuite))
}

// SetupTest initiates the test setups prior to each test
func (suite *TopicAwareTopologyTestSuite) SetupTest() {
	// we consider fanout as maximum number of connections the node allows to have
	// TODO: optimize value of fanout.
	suite.fanout = 100

	nClusters := 3
	nCollectors := 100
	nTotal := 1000

	collectors, _ := test.GenerateIDs(suite.T(), nCollectors, test.DryRun, unittest.WithRole(flow.RoleCollection))
	others, _ := test.GenerateIDs(suite.T(), nTotal, test.DryRun,
		unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.ids = append(others, collectors...)

	// mocks state for collector nodes topology
	suite.state, suite.clusters = topology.CreateMockStateForCollectionNodes(suite.T(),
		suite.ids.Filter(filter.HasRole(flow.RoleCollection)), uint(nClusters))

	// takes first id as the current nodes id
	suite.me = *suite.ids[0]
}

// TestTopologySize_Topic verifies that size of each topology fanout per topic is greater than
// `(k+1)/2` where `k` is number of nodes subscribed to a topic. It does that over 100 random iterations.
func (suite *TopicAwareTopologyTestSuite) TestTopologySize_Topic() {
	for i := 0; i < 100; i++ {
		top, err := topology.NewTopicBasedTopology(suite.me.NodeID, suite.state)
		require.NoError(suite.T(), err)

		topics := engine.ChannelIDsByRole(suite.me.Role)
		require.Greater(suite.T(), len(topics), 1)

		for _, topic := range topics {
			// extracts total number of nodes subscribed to topic
			roles, ok := engine.RolesByChannelID(topic)
			require.True(suite.T(), ok)

			ids, err := top.Subset(suite.ids, suite.fanout, topic)
			require.NoError(suite.T(), err)

			// counts total number of nodes that has the roles and are not `suite.me`  (node of interest).
			total := len(suite.ids.Filter(filter.And(filter.HasRole(roles...),
				filter.Not(filter.HasNodeID(suite.me.NodeID)))))
			require.True(suite.T(), float64(len(ids)) >= (float64)(total+1)/2)
		}
	}
}

// TestDeteministicity is a weak test that verifies the same seed generates the same topology for a topic.
//
// It also checks the topology against non-inclusion of the node itself in its own topology.
func (suite *TopicAwareTopologyTestSuite) TestDeteministicity() {
	top, err := topology.NewTopicBasedTopology(suite.me.NodeID, suite.state)
	require.NoError(suite.T(), err)

	topics := engine.ChannelIDsByRole(suite.me.Role)
	require.Greater(suite.T(), len(topics), 1)

	// for each topic samples 100 topologies
	// all topologies for a topic should be the same
	for _, topic := range topics {
		var previous, current []string
		for i := 0; i < 100; i++ {
			previous = current
			current = nil

			// generate a new topology with a the same ids, size and seed
			ids, err := top.Subset(suite.ids, suite.fanout, topic)
			require.NoError(suite.T(), err)

			// topology should not contain the node itself
			require.Empty(suite.T(), ids.Filter(filter.HasNodeID(suite.me.NodeID)))

			for _, v := range ids {
				current = append(current, v.NodeID.String())
			}
			// no guarantees about order is made by Topology.Subset(), hence sort the return values before comparision
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
func (suite *TopicAwareTopologyTestSuite) TestUniqueness() {
	var previous, current []string

	// for each topic samples 100 topologies
	// all topologies for a topic should be the same
	topics := engine.ChannelIDsByRole(flow.RoleConsensus)
	require.Greater(suite.T(), len(topics), 1)

	for _, identity := range suite.ids {
		// extracts all topics node (i) subscribed to
		if identity.Role != flow.RoleConsensus {
			continue
		}

		previous = current
		current = nil

		// creates and samples a new topic aware topology for the first topic of consensus nodes
		top, err := topology.NewTopicBasedTopology(identity.NodeID, suite.state)
		require.NoError(suite.T(), err)
		ids, err := top.Subset(suite.ids, suite.fanout, topics[0])
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
func (suite *TopicAwareTopologyTestSuite) TestConnectedness_NonClusterChannelID() {
	channelID := engine.TestNetwork
	// adjacency map keeps graph component of a single channel ID
	channelIDAdjMap := make(map[flow.Identifier]flow.IdentityList)

	for _, id := range suite.ids {
		// creates a topic-based topology for node
		top, err := topology.NewTopicBasedTopology(id.NodeID, suite.state)
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.Subset(suite.ids, suite.fanout, channelID)
		require.NoError(suite.T(), err)

		channelIDAdjMap[id.NodeID] = subset
	}

	topology.CheckConnectednessByChannelID(suite.T(), channelIDAdjMap, suite.ids, channelID)
}

// TestConnectedness_NonClusterChannelID checks whether graph components corresponding to a
// cluster channel ID are individually connected.
func (suite *TopicAwareTopologyTestSuite) TestConnectedness_ClusterChannelID() {
	// picks one cluster channel ID as sample
	channelID := clusterChannelIDs(suite.T())[0]

	// adjacency map keeps graph component of a single channel ID
	channelIDAdjMap := make(map[flow.Identifier]flow.IdentityList)

	// iterates over collection nodes
	for _, id := range suite.ids.Filter(filter.HasRole(flow.RoleCollection)) {
		// creates a channelID-based topology for node
		top, err := topology.NewTopicBasedTopology(id.NodeID, suite.state)
		require.NoError(suite.T(), err)

		// samples subset of topology
		subset, err := top.Subset(suite.ids, suite.fanout, channelID)
		require.NoError(suite.T(), err)

		channelIDAdjMap[id.NodeID] = subset
	}

	// check that each of the collection clusters forms a connected graph
	for _, cluster := range suite.clusters {
		suite.checkConnectednessByCluster(suite.T(), channelIDAdjMap, cluster)
	}
}

// clusterChannelIDs is a test helper method that returns all cluster-based channel ids.
func clusterChannelIDs(t *testing.T) []string {
	ccids := make([]string, 0)
	for _, channelID := range engine.ChannelIDs() {
		if _, ok := engine.IsClusterChannelID(channelID); !ok {
			continue
		}
		ccids = append(ccids, channelID)
	}

	require.NotEmpty(t, ccids)
	return ccids
}

// checkConnectednessByCluster is a test helper that checks all nodes belong to a cluster are connected.
func (suite *TopicAwareTopologyTestSuite) checkConnectednessByCluster(t *testing.T,
	adjMap map[flow.Identifier]flow.IdentityList,
	cluster flow.IdentityList) {
	topology.CheckGraphConnected(t,
		adjMap,
		suite.ids,
		filter.In(cluster))
}
