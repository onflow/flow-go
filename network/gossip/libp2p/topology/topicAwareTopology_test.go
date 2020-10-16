package topology_test

import (
	"os"
	"sort"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/test"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/utils/unittest"
)

// TopicAwareTopologyTestSuite tests the bare minimum requirements of a randomized
// topology that is needed for our network. It should not replace the information
// theory assumptions behind the schemes, e.g., random oracle model of hashes
type TopicAwareTopologyTestSuite struct {
	suite.Suite
	ids    flow.IdentityList // represents the identity list of all nodes in the system
	net    *libp2p.Network   // represents the single network instance that creates topology
	me     flow.Identity     // represents identity of single instance of node that creates topology
	fanout uint              // represents maximum number of connections this peer allows to have
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

	// creates 20 nodes of each type, 100 nodes overall.
	suite.ids = unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleCollection))
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleConsensus))...)
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleVerification))...)
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleExecution))...)
	suite.ids = append(suite.ids, unittest.IdentityListFixture(20, unittest.WithRole(flow.RoleAccess))...)

	// takes firs id as the current nodes id
	suite.me = *suite.ids[0]

	logger := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	key, err := test.GenerateNetworkingKey(suite.me.NodeID)
	require.NoError(suite.T(), err)

	// creates a middleware instance
	mw, err := libp2p.NewMiddleware(logger,
		json.NewCodec(),
		"0.0.0.0:0",
		suite.me.NodeID,
		key,
		metrics.NewNoopCollector(),
		libp2p.DefaultMaxUnicastMsgSize,
		libp2p.DefaultMaxPubSubMsgSize,
		unittest.IdentifierFixture().String())
	require.NoError(suite.T(), err)

	// creates and returns a topic aware topology instance
	top, err := topology.NewTopicAwareTopology(suite.me.NodeID)
	require.NoError(suite.T(), err)

	// creates a mock network with the topology instance
	nets, err := test.CreateNetworks(logger, []*libp2p.Middleware{mw}, suite.ids, 1, true, top)
	require.NoError(suite.T(), err)
	require.Len(suite.T(), nets, 1)
	suite.net = nets[0]
}

// TODO: fix this test after we have fanout optimized.
// TestTopologySize evaluates that overall topology size of a node is bound by its fanout.
func (suite *TopicAwareTopologyTestSuite) TestTopologySize() {
	suite.T().Skip("this test requires optimizing the fanout per topic")
	top, err := suite.net.Topology()
	require.NoError(suite.T(), err)
	require.Len(suite.T(), top, int(suite.fanout))
}

// TestMembership evaluates every id in topology to be a protocol id
func (suite *TopicAwareTopologyTestSuite) TestMembership() {
	top, err := suite.net.Topology()
	require.NoError(suite.T(), err)

	// every id in topology should be an id of the protocol
	for id := range top {
		require.Contains(suite.T(), suite.ids.NodeIDs(), id)
	}
}

// TestDeteministicity verifies that the same seed generates the same topology for a topic
func (suite *TopicAwareTopologyTestSuite) TestDeteministicity() {
	top, err := topology.NewTopicAwareTopology(suite.me.NodeID)
	require.NoError(suite.T(), err)

	topics := engine.GetTopicsByRole(suite.me.Role)
	require.Greater(suite.T(), len(topics), 1)

	// for each topic samples 100 topologies
	// all topologies for a topic should be the same
	for _, topic := range topics {
		var previous, current []string
		for i := 0; i < 100; i++ {
			previous = current
			current = nil

			// generate a new topology with a the same ids, size and seed
			idMap, err := top.Subset(suite.ids, suite.fanout, topic)
			require.NoError(suite.T(), err)

			for _, v := range idMap {
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
// Note: currently we are using a linear fanout for guaranteed delivery, hence there are
// C(n, (n+1)/2) many unique topologies for the same topic across different nodes. Even for small numbers
// like n = 300, the potential outcomes are large enough (i.e., 10e88) so that the uniqueness is guaranteed.
// This test however, performs a very weak uniqueness test by checking the uniqueness among consecutive topologies.
func (suite *TopicAwareTopologyTestSuite) TestUniqueness() {
	var previous, current []string

	// for each topic samples 100 topologies
	// all topologies for a topic should be the same
	topics := engine.GetTopicsByRole(flow.RoleConsensus)
	require.Greater(suite.T(), len(topics), 1)

	for i := 0; i < len(suite.ids); i++ {
		previous = current
		current = nil

		// extracts all topics node (i) subscribed to
		identity, _ := suite.ids.ByIndex(uint(i))
		if identity.Role != flow.RoleConsensus {
			continue
		}

		// creates and samples a new topic aware topology for the first topic of collection nodes
		top, err := topology.NewTopicAwareTopology(identity.NodeID)
		require.NoError(suite.T(), err)
		idMap, err := top.Subset(suite.ids, suite.fanout, topics[0])
		require.NoError(suite.T(), err)

		for _, v := range idMap {
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
